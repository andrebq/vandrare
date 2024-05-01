package ssh

import (
	"context"
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
	fmt.Fprintf(s.Stderr(), "Starting session: %v\n", time.Now())

	level := slog.LevelInfo
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(s.Stderr(), &slog.HandlerOptions{Level: level}))

	sh := appshell.New(true)

	echoMod := appshell.EchoModule(s, "echo")
	sh.AddModules(echoMod, g.keyManagementModule(s.Context()), g.tokenManagement(s.Context()))

	err := sh.EvalInteractive(s.Context(), s)
	if err != nil {
		log.Error("Error while processing code", "error", err)
		exitCode = 1
	}
	fmt.Fprintln(s)
}

func (g *Gateway) tokenManagement(ctx context.Context) *appshell.Module {
	mod := appshell.NewModule("tokenset")
	mod.AddFuncRaw("issue", appshell.FuncNR1(func(args ...string) (string, error) {
		owner := args[0]
		description := args[1]
		var err error
		ttl, err := time.ParseDuration(args[2])
		if err != nil {
			return "", err
		} else if ttl <= 0 {
			return "", errors.New("TTL must be positive, for lifetime access use issueLifetime")
		}
		token, err := g.tdb.Issue(ctx, owner, description, ttl)
		if err != nil {
			return "", err
		}
		return token, nil
	}))
	mod.AddFuncRaw("issueLifetime", appshell.FuncNR1(func(args ...string) (string, error) {
		owner := args[0]
		description := args[1]
		if len(args) != 2 {
			return "", errors.New("lifetime expects only two arguments")
		}
		token, err := g.tdb.Issue(ctx, owner, description, -1)
		if err != nil {
			return "", err
		}
		return token, nil
	}))
	mod.AddFuncRaw("listActive", appshell.FuncNR1Cast(func(args ...string) ([]TokenInfo, error) {
		owner := args[0]
		tokens, err := g.tdb.ListActive(ctx, owner)
		if err != nil {
			return nil, err
		}
		return tokens, nil
	}, appshell.FromInterfaceSlice[TokenInfo, []TokenInfo](appshell.ToFlatMap[TokenInfo]())))
	mod.AddFuncRaw("revoke", appshell.FuncNR0(func(args ...string) error {
		id := args[0]
		return g.tdb.Revoke(ctx, id)
	}))
	return mod
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
