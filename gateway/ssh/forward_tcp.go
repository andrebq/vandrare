package ssh

import (
	"fmt"
	"log/slog"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

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
