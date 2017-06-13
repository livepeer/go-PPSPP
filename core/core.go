package core

type MsgError struct{}

type ChanID uint8

type Opcode uint8

const (
	HANDSHAKE Opcode = 1
)

type Msg struct {
	op   Opcode
	data interface{}
}

type Handshake struct {
	chanID ChanID
	// TODO: swarm SwarmMetadata
	// TODO: peer capabilities
}

type Datagram struct {
	chanID ChanID
	msgs   []Msg
}

func handleDatagram(d Datagram) error {
	return nil
}
