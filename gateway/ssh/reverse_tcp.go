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

func (g *Gateway) handleTCPForward(sshctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
	if !g.ensurePubkeyAuth(sshctx) {
		return false, nil
	}
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

func (g *Gateway) handleCancelTCPForward(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
	if !g.ensurePubkeyAuth(ctx) {
		return false, nil
	}
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
