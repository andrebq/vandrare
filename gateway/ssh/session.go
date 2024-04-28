package ssh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/andrebq/vandrare/internal/pattern"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

var (
	whoamiCmd = pattern.Prefix([]string{"vandrare", "whoami"}, nil)
)

func (g *Gateway) sessionHandler(s ssh.Session) {
	if g.isAdminSession(s) {
		slog.Info("Starting admin session", "command", s.Command(), "pubkey", string(gossh.MarshalAuthorizedKey(s.PublicKey())), "user", s.User(), "addr", s.RemoteAddr())
		g.runAdminSession(s)
		return
	}
	switch {
	case pattern.Match(s.Command(), whoamiCmd):
		g.sessionHandleWhoami(s)
	}
	fmt.Fprintf(s, "Successful authentication, but your credentials do not allow interactive access\n")
	s.Exit(0)
	s.Close()
}

func (g *Gateway) sessionHandleWhoami(s ssh.Session) {
	json.NewEncoder(s).Encode(struct {
		User        string    `json:"user"`
		Key         string    `json:"key"`
		Fingerprint string    `json:"fingerprint"`
		Now         time.Time `json:"now"`
	}{
		User:        s.User(),
		Key:         string(bytes.TrimSpace(gossh.MarshalAuthorizedKey(s.PublicKey()))),
		Fingerprint: gossh.FingerprintSHA256(s.PublicKey()),
		Now:         time.Now(),
	})
	s.Exit(0)
}
