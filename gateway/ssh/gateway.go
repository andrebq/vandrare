package ssh

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type (
	Gateway struct {
		l         sync.Mutex
		accepting map[string]*listener
	}

	listener struct {
		incoming chan connData
		cancel   context.CancelFunc
		ctx      context.Context
	}

	connData struct {
		io   io.ReadWriteCloser
		addr struct {
			src  string
			port uint32
		}
	}
)

func NewGateway() *Gateway {
	return &Gateway{
		accepting: make(map[string]*listener),
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

	srv.Handler = func(s ssh.Session) {
		io.Copy(io.Discard, s)
	}
	err := srv.ListenAndServe()
	return err
}

func (g *Gateway) handleDirectTCPIP(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
	defer func() {
		if conn != nil {
			println("wtf!")
			conn.Close()
		}
	}()

	slog.Info("Direct TCP/IP connection", "conn", conn.RemoteAddr(), "channel", newChan.ChannelType())
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
	slog.Info("Attempting local connection", "data", data)
	identity := fmt.Sprintf("%v:%v", data.DestAddr, data.DestPort)
	g.l.Lock()
	lst := g.accepting[identity]
	g.l.Unlock()
	if lst == nil {
		slog.Debug("Listener not found", "identity", identity)
		newChan.Reject(gossh.ConnectionFailed, "listener not found")
		ch.Close()
		return
	}
	wrapConn := connData{
		io: ch,
	}
	wrapConn.addr.src = data.OriginAddr
	wrapConn.addr.port = data.OriginPort

	select {
	case lst.incoming <- wrapConn:
		// no need to close it
		conn = nil
	case <-lst.ctx.Done():
		slog.Debug("Listener closed", "identity", identity)
		newChan.Reject(gossh.ConnectionFailed, "listener done")
		ch.Close()
		return
	case <-ctx.Done():
		ch.Close()
	}
}

func (g *Gateway) handleTCPForward(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
	conn := ctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn)

	var reqPayload remoteForwardRequest
	if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
		slog.Debug("Parsing error for tcp forwarding", "err", err)
		return false, []byte{}
	}
	identity := fmt.Sprintf("%v:%v", reqPayload.BindAddr, reqPayload.BindPort)
	incoming := make(chan connData, 1)
	listenerCtx, cancel := context.WithCancel(ctx)

	g.l.Lock()
	g.accepting[identity] = &listener{
		incoming: incoming,
		cancel:   cancel,
		ctx:      listenerCtx,
	}
	g.l.Unlock()
	slog.Info("Accepting new connections for server", "address", identity)

	go func() {
	LISTENER:
		for {

			var cliConn connData
			select {
			case cliConn = <-incoming:
			case <-ctx.Done():
				break LISTENER
			}
			slog.Debug("New Connection")

			payload := gossh.Marshal(&remoteForwardChannelData{
				DestAddr:   reqPayload.BindAddr,
				DestPort:   reqPayload.BindPort,
				OriginAddr: "127.0.0.2",
				OriginPort: 10000,
			})
			go func() {
				ch, reqs, err := conn.OpenChannel(forwardedTCPChannelType, payload)
				if err != nil {
					slog.Debug("Unable to open channel to reverse", "err", err)
					cliConn.io.Close()
				}
				go gossh.DiscardRequests(reqs)
				go func() {
					slog.Debug("Copying data from connection to channel")
					defer ch.Close()
					defer cliConn.io.Close()
					io.Copy(ch, cliConn.io)
				}()
				go func() {
					slog.Debug("Copying data from channel to connection")
					defer ch.Close()
					defer cliConn.io.Close()
					io.Copy(cliConn.io, ch)
				}()
			}()
		}
		g.l.Lock()
		g.accepting[identity] = nil
		g.l.Unlock()
	}()
	return true, gossh.Marshal(&remoteForwardSuccess{reqPayload.BindPort})
}

func (g *Gateway) handleCancelTCPForward(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
	var reqPayload remoteForwardCancelRequest
	if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
		return false, []byte{}
	}
	identity := fmt.Sprintf("%v:%v", reqPayload.BindAddr, reqPayload.BindPort)
	g.l.Lock()
	lst := g.accepting[identity]
	g.l.Unlock()
	lst.cancel()
	return false, []byte{}
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
