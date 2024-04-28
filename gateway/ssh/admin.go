package ssh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/andrebq/vandrare/internal/appshell"
	"github.com/andrebq/vandrare/internal/pattern"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

func (g *Gateway) isAdminUser(ctx ssh.Context) bool {
	val, found := ctx.Permissions().Extensions["allow_admin"]
	return found && val == "true"
}

func (g *Gateway) isAdminSession(s ssh.Session) bool {
	return g.isAdminUser(s.Context()) && pattern.Match(s.Command(), vandrareAdminCommand)
}

func (g *Gateway) runAdminSession(s ssh.Session) {
	exitCode := 0
	defer func() {
		s.Exit(exitCode)
		s.Context().Value(ssh.ContextKeyConn).(*gossh.ServerConn).Close()
	}()
	fmt.Fprintf(s, "Starting session: %v\n", time.Now())

	level := slog.LevelInfo
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(s.Stderr(), &slog.HandlerOptions{Level: level}))

	sh := appshell.New(true)

	echoMod := appshell.NewModule("echo")
	echoMod.AddFuncRaw("print", appshell.DynFuncNR0(func(args ...any) error {
		strs := make([]string, len(args))
		for i, v := range args {
			strs[i] = fmt.Sprintf("%v", v)
		}
		return json.NewEncoder(s).Encode(strs)
	}))
	sh.AddModules(echoMod, g.keyManagementModule(s.Context()))

	err := sh.EvalInteractive(s.Context(), s)
	if err != nil {
		log.Error("Error while processing code", "error", err)
		exitCode = 1
	}
	fmt.Fprintln(s)
}

func (g *Gateway) keyManagementModule(ctx context.Context) *appshell.Module {
	mod := appshell.NewModule("keyset")
	mod.AddFuncRaw("put", appshell.FuncNR0(func(args ...string) error {
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(args[0]))
		if err != nil {
			return err
		}
		validFromDur, err := time.ParseDuration(args[1])
		if err != nil {
			return err
		}
		expiresInDur, err := time.ParseDuration(args[2])
		if err != nil {
			return err
		}
		hostname := args[3]
		if len(hostname) == 0 {
			return errors.New("invalid hostname")
		}
		err = g.kdb.RegisterKey(ctx, key, time.Now().Add(validFromDur), time.Now().Add(expiresInDur), []string{hostname})
		slog.Info("Key registration", "key", string(gossh.MarshalAuthorizedKey(key)), "hostname", hostname, "err", err)
		return err
	}))

	mod.AddFuncRaw("addPermission", appshell.FuncNR0(func(args ...string) error {
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(args[0]))
		if err != nil {
			return err
		}
		operation := args[1]
		resource := args[2]
		action := strings.ToLower(args[3])
		switch action {
		case "allow", "deny":
		default:
			return fmt.Errorf("invalid action: %v", action)
		}
		err = g.kdb.SetPermission(ctx, key, operation, resource, action)
		slog.Info("Key authorization", "key", string(gossh.MarshalAuthorizedKey(key)), "operation", operation, "resource", resource, "action", action, "err", err)
		return err
	}))
	return mod
}
