package ssh

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/andrebq/vandrare/internal/loadbalancer"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type (
	Gateway struct {
		l         sync.Mutex
		accepting map[string]*loadbalancer.LB[connData]
		cleanup   map[*gossh.ServerConn]func()
		kdb       KeyDB
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

func NewGateway(keydb KeyDB) *Gateway {
	return &Gateway{
		kdb:       keydb,
		accepting: make(map[string]*loadbalancer.LB[connData]),
		cleanup:   make(map[*gossh.ServerConn]func()),
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

	srv.Handler = func(s ssh.Session) {
		io.Copy(io.Discard, s)
	}
	err := srv.ListenAndServe()
	return err
}

func (g *Gateway) handleDirectTCPIP(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	slog.Debug("Direct TCP/IP connection", "conn", conn.RemoteAddr(), "channel", newChan.ChannelType())
	data := struct {
		DestAddr string
		DestPort uint32

		OriginAddr string
		OriginPort uint32
	}{}

	if err := gossh.Unmarshal(newChan.ExtraData(), &data); err != nil {
		slog.Error("Unable to parse connection target", "err", err)
		newChan.Reject(gossh.ConnectionFailed, "invalid data from client")
		return
	}

	ch, reqs, err := newChan.Accept()
	if err != nil {
		slog.Error("Unable to accept channel", "err", err)
		newChan.Reject(gossh.ConnectionFailed, "invalid data from client")
		return
	}

	go gossh.DiscardRequests(reqs)
	slog.Debug("Attempting local connection", "data", data)
	identity := fmt.Sprintf("%v:%v", data.DestAddr, data.DestPort)
	lb := g.getLB(identity)
	if lb == nil {
		slog.Debug("Listener not found", "identity", identity)
		newChan.Reject(gossh.ConnectionFailed, "listener not found")
		ch.Close()
		return
	}
	wrapConn := connData{
		io: ch,
	}
	wrapConn.from.host = data.OriginAddr
	wrapConn.from.port = data.OriginPort
	wrapConn.to.host = data.DestAddr
	wrapConn.to.port = data.DestPort

	err = lb.Offer(ctx, wrapConn)
	if err != nil {
		slog.Debug("Unable to schedule work", "err", err)
		ch.Close()
		return
	}
	conn = nil
}

func (g *Gateway) handleTCPForward(sshctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
	// register a new listener for a given endpoint
	// wait for new connections from the load balancer
	// handle each connection in a separate thread
	// cleanup once the context is closed
	ctx, connections, boundPort, cleanup, err := g.registerEndpoint(sshctx, req)
	if err != nil {
		slog.Debug("Unable to peform endpoint registration", "err", err)
		return false, []byte{}
	}
	go func() {
		defer cleanup()
		for {
			select {
			case conn, open := <-connections:
				if !open {
					return
				}
				go g.handleReverseConnection(ctx, conn)
			case <-ctx.Done():
				return
			}
		}
	}()
	return true, gossh.Marshal(&remoteForwardSuccess{boundPort})
}

func (g *Gateway) handleReverseConnection(ctx context.Context, conn connData) {
	payload := gossh.Marshal(&remoteForwardChannelData{
		DestAddr:   conn.to.host,
		DestPort:   conn.to.port,
		OriginAddr: conn.from.host,
		OriginPort: conn.from.port,
	})
	sshconn := ctx.Value(ssh.ContextKeyConn)
	if sshconn == nil {
		return
	}
	ch, reqs, err := sshconn.(*gossh.ServerConn).OpenChannel(forwardedTCPChannelType, payload)
	if err != nil {
		slog.Debug("Unable to open channel to reverse", "err", err)
		conn.io.Close()
		return
	}
	go gossh.DiscardRequests(reqs)
	go copyAndClose(ch, conn.io)
	go copyAndClose(conn.io, ch)
}

func copyAndClose(to io.WriteCloser, from io.ReadCloser) {
	defer to.Close()
	defer from.Close()
	io.Copy(to, from)
}

func (g *Gateway) registerEndpoint(sshctx ssh.Context, req *gossh.Request) (context.Context, <-chan connData, uint32, func(), error) {
	var reqPayload remoteForwardRequest
	if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
		return nil, nil, 0, nil, fmt.Errorf("ssh-gateway: remote forward parse error: %w", err)
	}
	identity := fmt.Sprintf("%v:%v", reqPayload.BindAddr, reqPayload.BindPort)

	lb := g.acquireLB(identity)
	connections := lb.New()

	ctx, cancel := context.WithCancel(sshctx)

	cleanup := sync.OnceFunc(func() {
		g.l.Lock()
		lb.Remove(connections)
		if lb.Empty() {
			delete(g.accepting, identity)
		}
		g.l.Unlock()
		sshctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn).Close()
		cancel()
	})

	g.registerCleanup(sshctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn), cleanup)

	return ctx, connections, reqPayload.BindPort, cleanup, nil
}

func (g *Gateway) acquireLB(endpoint string) *loadbalancer.LB[connData] {
	g.l.Lock()
	defer g.l.Unlock()
	lb := g.accepting[endpoint]
	if lb == nil {
		lb = loadbalancer.NewLB[connData](context.Background(), time.Now().Unix())
		g.accepting[endpoint] = lb
	}
	return lb
}

func (g *Gateway) getLB(endpoint string) *loadbalancer.LB[connData] {
	g.l.Lock()
	defer g.l.Unlock()
	lb := g.accepting[endpoint]
	return lb
}

func (g *Gateway) handleCancelTCPForward(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
	var reqPayload remoteForwardCancelRequest
	if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
		slog.Debug("Erro while decoding remote forward cancel", "err", err)
		return false, []byte{}
	}
	cleanup := g.getCleanup(ctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn))
	if cleanup != nil {
		cleanup()
	}
	return false, []byte{}
}

func (g *Gateway) registerCleanup(conn *gossh.ServerConn, cleanup func()) {
	g.l.Lock()
	g.cleanup[conn] = cleanup
	g.l.Unlock()
}

func (g *Gateway) getCleanup(conn *gossh.ServerConn) func() {
	g.l.Lock()
	cl := g.cleanup[conn]
	g.l.Unlock()
	return cl
}

type remoteForwardChannelData struct {
	DestAddr   string
	DestPort   uint32
	OriginAddr string
	OriginPort uint32
}
type remoteForwardSuccess struct {
	BindPort uint32
}

type remoteForwardRequest struct {
	BindAddr string
	BindPort uint32
}

type remoteForwardCancelRequest struct {
	BindAddr string
	BindPort uint32
}

const (
	forwardedTCPChannelType = "forwarded-tcpip"
)
