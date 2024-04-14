package protocol

type (
	MessageType byte

	ID struct {
		Endpoint string `msgpack:"e"`
		RemoteID uint64 `msgpack:"i"`
	}
	Packet struct {
		To      uint64 `msgpack:"t"`
		From    uint64 `msgpack:"f"`
		Payload []byte `msgpack:"b"`
	}

	Message struct {
		Type     MessageType `msgpack:"t"`
		Identity *ID         `msgpack:"id"`
		Packet   *Packet     `msgpack:"p"`
	}
)

const (
	Undefined = MessageType(iota)
	Identity
	Listen
	Dial

	Connect
	IO
)
