package ssh

import (
	"bufio"
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
	defer func() {
		s.Close()
		s.Context().Value(ssh.ContextKeyConn).(*gossh.ServerConn).Close()
	}()

	level := slog.LevelInfo
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(s.Stderr(), &slog.HandlerOptions{Level: level}))

	sh := appshell.New(true)

	echoMod := appshell.NewModule("echo")
	echoMod.AddFuncRaw("print", appshell.DynFuncNR0(func(args ...any) error {
		return json.NewEncoder(s).Encode(args)
	}))
	sh.AddModules(echoMod, g.keyManagementModule(s.Context()))

	sc := bufio.NewScanner(s)
	acc := strings.Builder{}
	for sc.Scan() {
		fmt.Fprintln(&acc, sc.Text())
		if sh.ValidScript(acc.String()) {
			output, err := sh.Eval(s.Context(), acc.String())
			if err != nil {
				log.Error("Error processing script", "err", err)
				return
			}
			acc.Reset()
			if output != nil {
				json.NewEncoder(s).Encode(output)
			}
		}
	}
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
	return mod
}
