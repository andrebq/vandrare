package bridge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/andrebq/vandrare/gateway/quick/bridge/protocol"
	"github.com/andrebq/vandrare/internal/stack"
	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"
)

type (
	// WSBridge allows nodes to expose themselves as `quick endpoints` which allows full
	// duplex communication between nodes connected to the same bridge
	//
	// Client connections can then behave as quick-servers as well as quick-clients,
	// the same endpoint can open multiple web-socket connections, thus avoiding most latency issues
	// associated with multiplexing connections over a single TCP connection.
	WSBridge struct {
		mutex   sync.Mutex
		upgrade websocket.Upgrader

		nextid uint64
		addr   string

		endpoints map[string]uint64
		channels  map[uint64]*stack.S[*wsIO]
	}

	wsIO struct {
		mutex      sync.Mutex
		upstream   <-chan wsPacket
		downstream chan<- wsPacket
		err        error
	}

	wsPacket struct {
		mt  int
		buf []byte
	}
)

var (
	errMissingidentity       = errors.New("wsbridge: identity message not received")
	errUnexpectedMessageType = errors.New("wsbridge: unexpected message type")
	errDisconnected          = errors.New("wsbridge: disconnected")
)

func NewWebsocket(addr string) *WSBridge { return &WSBridge{addr: addr} }

func (ws *WSBridge) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	srv := http.Server{
		BaseContext: func(l net.Listener) context.Context {
			go func() {
				<-ctx.Done()
				l.Close()
			}()
			return ctx
		},
		Addr: ws.addr,
	}
	srv.Handler = http.HandlerFunc(ws.handleSocket)
	return srv.ListenAndServe()
}

func (ws *WSBridge) handleSocket(w http.ResponseWriter, req *http.Request) {
	conn, err := ws.upgrade.Upgrade(w, req, nil)
	if err != nil {
		slog.Debug("Unable to upgrade client connection", "err", err)
		return
	}
	defer conn.Close()
	wio := wsIO{}
	ctx, cancel := context.WithCancelCause(req.Context())
	defer cancel(context.Canceled)

	wio.upstream = sinkConn(ctx, conn, cancel)
	wio.downstream = sourceConn(ctx, conn, cancel)

	var msg protocol.Message
	wio.read(ctx, &msg)
	if msg.Type != protocol.Identity {
		cancel(errMissingidentity)
		return
	}

	msg.Identity.RemoteID = ws.registerIO(msg.Identity.Endpoint, &wio)
	defer ws.unregisterIO(msg.Identity.Endpoint, &wio)
	wio.write(ctx, &msg)

	identity := *msg.Identity

	if wio.failed() {
		return
	}

	wio.read(ctx, &msg)
	switch msg.Type {
	case protocol.Listen:
		cancel(ws.handleListener(ctx, identity, &wio))
	case protocol.Dial:
		ws.handleDialer(ctx, identity, &wio)
	default:
		slog.Debug("Unexpected message type", "mt", msg.Type)
		return
	}
}

func (ws *WSBridge) handleListener(ctx context.Context, self protocol.ID, wio *wsIO) error {
	for {
		var msg protocol.Message
		wio.read(ctx, &msg)
		switch msg.Type {
		case protocol.IO:
			cliwio := ws.findChannel(msg.Packet.To)
			if cliwio == nil {
				// drop
				continue
			}
			msg.Packet.From = self.RemoteID
			cliwio.write(ctx, &msg)
		default:
			slog.Debug("Unexpected message type from Listener", "mt", msg.Type, "endpoint", self.Endpoint, "id", self.RemoteID)
			return errUnexpectedMessageType
		}
	}
}

func (ws *WSBridge) handleDialer(ctx context.Context, self protocol.ID, wio *wsIO) error {
	for !wio.failed() {
		var msg protocol.Message
		wio.read(ctx, &msg)
		switch msg.Type {
		case protocol.Connect:
			remote, found := ws.findEndpoint(msg.Identity.Endpoint)
			if !found {
				msg.Identity.RemoteID = 0
				msg.Identity.Endpoint = ""
			} else {
				msg.Identity.RemoteID = remote
			}
			*msg.Identity = self
			remio := ws.findChannel(remote)
			if remio == nil {
				wio.fail(errDisconnected)
			}
		case protocol.IO:
			remio := ws.findChannel(msg.Packet.To)
			if remio == nil {
				wio.fail(errDisconnected)
			}
			msg.Packet.From = self.RemoteID
			remio.write(ctx, &msg)
		}
	}

	return wio.err
}

func (ws *WSBridge) findEndpoint(endpoint string) (remote uint64, found bool) {
	ws.mutex.Lock()
	remote, found = ws.endpoints[endpoint]
	ws.mutex.Unlock()
	return
}

func (ws *WSBridge) findChannel(remote uint64) *wsIO {
	ws.mutex.Lock()
	channels := ws.channels[remote]
	if channels == nil {
		return nil
	}
	defer ws.mutex.Unlock()
	wio := channels.Pop()
	channels.Push(wio)
	return wio
}

func (ws *WSBridge) unregisterIO(endpoint string, io *wsIO) {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()
	if ws.endpoints == nil {
		return
	}
	remote, found := ws.endpoints[endpoint]
	if !found {
		return
	}
	st := ws.channels[remote]
	st.Discard(io)
	if st.Empty() {
		delete(ws.endpoints, endpoint)
		delete(ws.channels, remote)
	}
}

func (ws *WSBridge) registerIO(endpoint string, io *wsIO) uint64 {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	if ws.endpoints == nil {
		ws.channels = make(map[uint64]*stack.S[*wsIO])
		ws.endpoints = make(map[string]uint64)
	}
	if id, found := ws.endpoints[endpoint]; found {
		return id
	}
	ws.nextid++
	st := &stack.S[*wsIO]{}
	ws.endpoints[endpoint] = ws.nextid
	ws.channels[ws.nextid] = st
	st.Push(io)
	return ws.nextid
}

func sinkConn(ctx context.Context, conn *websocket.Conn, cancel context.CancelCauseFunc) <-chan wsPacket {
	const maxTTL = time.Minute
	ch := make(chan wsPacket, 1)
	go func() {
		for {
			conn.SetReadDeadline(time.Now().Add(maxTTL))
			mt, buf, err := conn.ReadMessage()
			if err != nil {
				cancel(fmt.Errorf("wsbridge: IOError: %w", err))
				return
			}
			select {
			case <-ctx.Done():
				cancel(ctx.Err())
				return
			case ch <- wsPacket{mt: mt, buf: buf}:
			}
		}
	}()
	return ch
}

func sourceConn(ctx context.Context, conn *websocket.Conn, cancel context.CancelCauseFunc) chan<- wsPacket {
	const maxTTL = time.Minute
	ch := make(chan wsPacket, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				cancel(ctx.Err())
				return
			case wp := <-ch:
				conn.SetWriteDeadline(time.Now().Add(maxTTL))
				err := conn.WriteMessage(wp.mt, wp.buf)
				if err != nil {
					cancel(fmt.Errorf("wsbridge: IOError: %w", err))
					return
				}
			}
		}
	}()
	return ch
}

func (wio *wsIO) write(ctx context.Context, out *protocol.Message) {
	buf, err := msgpack.Marshal(out)
	if err != nil {
		wio.fail(err)
	}
	if wio.failed() {
		return
	}
	wio.send(ctx, wsPacket{mt: websocket.BinaryMessage, buf: buf})
}

func (wio *wsIO) read(ctx context.Context, out *protocol.Message) {
	if wio.failed() {
		*out = protocol.Message{}
		return
	}
	var packet wsPacket
	if !wio.recv(ctx, &packet) {
		return
	}

	if packet.mt == websocket.PingMessage {
		wio.pong(ctx)
		return
	}

	err := msgpack.Unmarshal(packet.buf, out)
	if err != nil {
		wio.fail(err)
	}
}

func (wio *wsIO) pong(ctx context.Context) {
	wio.send(ctx, wsPacket{mt: websocket.PongMessage})
}

func (wio *wsIO) ping(ctx context.Context) {
	wio.send(ctx, wsPacket{mt: websocket.PingMessage})
}

func (wio *wsIO) recv(ctx context.Context, ws *wsPacket) bool {
	select {
	case <-ctx.Done():
		wio.fail(ctx.Err())
		return false
	case *ws = <-wio.upstream:
		return true
	}
}

func (wio *wsIO) send(ctx context.Context, ws wsPacket) bool {
	select {
	case <-ctx.Done():
		wio.fail(ctx.Err())
		return false
	case wio.downstream <- ws:
		return true
	}
}

func (wio *wsIO) failed() bool {
	wio.mutex.Lock()
	failed := wio.err != nil
	wio.mutex.Unlock()
	return failed
}

func (wio *wsIO) fail(err error) {
	wio.mutex.Lock()
	if wio.err == nil {
		wio.err = err
	}
	wio.mutex.Unlock()
}
