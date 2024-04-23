package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/andrebq/vandrare/internal/loadbalancer"
	"github.com/andrebq/vandrare/internal/pattern"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type (
	Gateway struct {
		l         sync.Mutex
		accepting map[string]*loadbalancer.LB[connData]
		cleanup   map[*gossh.ServerConn]func()
		kdb       *DynKDB
		adminKey  ssh.PublicKey
	}

	connData struct {
		io io.ReadWriteCloser
		to struct {
			host string
			port uint32
		}
		from struct {
			host string
			port uint32
		}
	}

	KeyDB interface {
		AuthN(ctx context.Context, key ssh.PublicKey) error
		AuthZ(ctx context.Context, key ssh.PublicKey, action, resource string) error
	}
)

var (
	vandrareAdminCommand = pattern.Prefix([]string{"vandrare", "gateway", "ssh", "admin"}, nil)
)

func NewGateway(keydb *DynKDB, adminKey ssh.PublicKey) *Gateway {
	return &Gateway{
		kdb:       keydb,
		accepting: make(map[string]*loadbalancer.LB[connData]),
		cleanup:   make(map[*gossh.ServerConn]func()),

		adminKey: adminKey,
	}
}

func (g *Gateway) Run(ctx context.Context, bind string) error {
	srv := ssh.Server{
		Addr: bind,
	}
	srv.ChannelHandlers = map[string]ssh.ChannelHandler{
		"session":      ssh.DefaultSessionHandler,
		"direct-tcpip": g.handleDirectTCPIP,
	}

	srv.RequestHandlers = map[string]ssh.RequestHandler{
		"tcpip-forward":        g.handleTCPForward,
		"cancel-tcpip-forward": g.handleCancelTCPForward,
	}

	srv.PasswordHandler = func(ctx ssh.Context, password string) bool { return false }
	srv.KeyboardInteractiveHandler = func(ctx ssh.Context, challenger gossh.KeyboardInteractiveChallenge) bool { return false }
	srv.PublicKeyHandler = func(ctx ssh.Context, key ssh.PublicKey) bool {
		perm := ctx.Permissions()
		if perm.Extensions == nil {
			perm.Extensions = make(map[string]string)
		}
		if bytes.Equal(key.Marshal(), g.adminKey.Marshal()) {
			perm.Extensions["allow_admin"] = "true"
			return true
		}
		if key.Type() != "ssh-ed25519" {
			return false
		}

		err := g.kdb.AuthN(ctx, key)
		if err != nil {
			slog.Debug("Authentication failed", "err", err)
			return false
		}
		return true
	}

	srv.PtyCallback = func(ctx ssh.Context, pty ssh.Pty) bool { return false }

	srv.Handler = func(s ssh.Session) {
		if g.isAdminSession(s) {
			slog.Info("Starting admin session", "command", s.Command(), "pubkey", string(gossh.MarshalAuthorizedKey(s.PublicKey())), "user", s.User(), "addr", s.RemoteAddr())
			g.runAdminSession(s)
			return
		}
		fmt.Fprintf(s, "Successful authentication, but your credentials do not allow interactive access\n")
		s.Exit(0)
		s.Close()
	}
	err := srv.ListenAndServe()
	return err
}
