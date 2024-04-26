package ssh

import (
	"fmt"
	"log/slog"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

func (g *Gateway) sessionHandler(s ssh.Session) {
	if g.isAdminSession(s) {
		slog.Info("Starting admin session", "command", s.Command(), "pubkey", string(gossh.MarshalAuthorizedKey(s.PublicKey())), "user", s.User(), "addr", s.RemoteAddr())
		g.runAdminSession(s)
		return
	}
	fmt.Fprintf(s, "Successful authentication, but your credentials do not allow interactive access\n")
	s.Exit(0)
	s.Close()
}
