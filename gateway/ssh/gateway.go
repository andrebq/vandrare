package ssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/andrebq/maestro"
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
		tdb       *TokenDB
		adminKey  ssh.PublicKey
		host      struct {
			key  ssh.Signer
			cert *gossh.Certificate
		}
		cakey    CAKey
		casigner gossh.Signer
		Binding  struct {
			SSH     string
			HTTP    string
			Domains []string
		}
		Subdomains []string
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

	// CAKey is simply a wrapper to a valid ssh key
	CAKey struct {
		actual ed25519.PrivateKey
	}

	KeyDB interface {
		AuthN(ctx context.Context, key ssh.PublicKey) error
		AuthZ(ctx context.Context, key ssh.PublicKey, action, resource string) error
	}

	ctxKey byte
)

const (
	pubkeyAuthKey = ctxKey(iota + 1)
)

var (
	vandrareAdminCommand = pattern.Prefix([]string{"vandrare", "gateway", "ssh", "admin"}, nil)
)

func GenerateCAKey(seed [ed25519.SeedSize]byte) CAKey {
	pk := ed25519.NewKeyFromSeed(seed[:])
	return CAKey{actual: pk}
}

func genCASigner(key CAKey) (ssh.Signer, error) {
	signerkey, err := gossh.NewSignerFromKey(key.actual)
	if err != nil {
		return nil, err
	}
	return signerkey, nil
}

func NewGateway(keydb *DynKDB, tkdb *TokenDB, adminKey ssh.PublicKey, cakey CAKey) (*Gateway, error) {
	casigner, err := genCASigner(cakey)
	if err != nil {
		return nil, err
	}
	g := &Gateway{
		kdb:       keydb,
		tdb:       tkdb,
		accepting: make(map[string]*loadbalancer.LB[connData]),
		cleanup:   make(map[*gossh.ServerConn]func()),

		cakey:    cakey,
		casigner: casigner,

		adminKey: adminKey,
	}
	return g, nil
}

// SetAdminToken sets the initial admin token if this is the first the token is being set,
// otherwise nothing happens
func (g *Gateway) SetAdminToken(ctx context.Context, token *[32]byte, ttl time.Duration) (bool, error) {
	return false, errors.New("not implemented")
}

func (g *Gateway) Run(ctx context.Context) error {
	mctx := maestro.New(ctx)
	if g.Binding.SSH != "" {
		mctx.Spawn(func(ctx maestro.Context) error {
			defer mctx.Shutdown()
			return g.runSSHD(ctx)
		})
	}
	if g.Binding.HTTP != "" {
		mctx.Spawn(func(ctx maestro.Context) error {
			defer mctx.Shutdown()
			return g.runHTTPD(ctx)
		})
	}
	// block forever until both children dies
	return mctx.WaitChildren(nil)
}

func (g *Gateway) runSSHD(ctx maestro.Context) error {
	var err error
	g.host.key, err = g.genHostKey()
	if err != nil {
		return err
	}
	srv := ssh.Server{
		Addr: g.Binding.SSH,
	}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	srv.AddHostKey(g.host.key)
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
			ctx.SetValue(pubkeyAuthKey, true)
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
		ctx.SetValue(pubkeyAuthKey, true)
		return true
	}
	srv.PtyCallback = func(ctx ssh.Context, pty ssh.Pty) bool { return false }
	srv.Handler = g.sessionHandler
	slog.Info("Starting SSHD server", "addr", srv.Addr)
	err = srv.ListenAndServe()
	ctx.Shutdown()
	return err
}

func (g *Gateway) genHostKey() (gossh.Signer, error) {
	pubkey, privkey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("gateway: unable to generate host key: %w", err)
	}
	sshpubKey, err := gossh.NewPublicKey(pubkey)
	if err != nil {
		return nil, fmt.Errorf("gateway: unable to generate host pub key: %w", err)
	}
	principals := map[string]struct{}{}
	for _, d := range g.Binding.Domains {
		principals[d] = struct{}{}
	}
	cert := &gossh.Certificate{
		KeyId:       g.Binding.Domains[0],
		Key:         sshpubKey,
		CertType:    gossh.HostCert,
		ValidAfter:  uint64(time.Now().Add(1 - time.Second).Unix()),
		ValidBefore: uint64(time.Now().Add(time.Hour * 24 * 365).Unix()),
	}
	for k := range principals {
		host, _, err := net.SplitHostPort(k)
		if err != nil || host == "" {
			continue
		}
		principals[host] = struct{}{}
	}

	cert.ValidPrincipals = make([]string, 0, len(principals))
	for k := range principals {
		cert.ValidPrincipals = append(cert.ValidPrincipals, k)
	}
	sort.Strings(cert.ValidPrincipals)

	if err := cert.SignCert(rand.Reader, g.casigner); err != nil {
		return nil, fmt.Errorf("gateway: unable to sign host certificate: %w", err)
	}

	g.host.cert = cert

	keysigner, err := gossh.NewSignerFromKey(privkey)
	if err != nil {
		return nil, fmt.Errorf("gateway: unable to generate host-key signer: %w", err)
	}
	certsigner, err := gossh.NewCertSigner(cert, keysigner)
	if err != nil {
		return nil, fmt.Errorf("gateway: unable to generate cert host signer: %w", err)
	}

	slog.Info("Host signer created", "cert", gossh.MarshalAuthorizedKey(cert),
		"signkey", gossh.FingerprintSHA256(cert.SignatureKey),
		"certkey", gossh.FingerprintSHA256(certsigner.PublicKey()))
	return certsigner, nil
}

func (g *Gateway) ensurePubkeyAuth(ctx ssh.Context) bool {
	if ctx.Value(pubkeyAuthKey) != nil {
		return true
	}
	ctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn).Close()
	return false
}
