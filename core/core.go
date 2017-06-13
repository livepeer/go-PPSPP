package core

import (
	"fmt"
)

type MsgError struct {
	c    ChanID
	m    Msg
	info string
}

func (e MsgError) Error() string {
	return fmt.Sprintf("message error on channel %v: %v", e.c, e.info)
}

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
	for _, msg := range d.msgs {
		err := handleMsg(d.chanID, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func handleMsg(c ChanID, m Msg) error {
	switch m.op {
	case HANDSHAKE:
		if c != 0 {
			return MsgError{c: c, m: m, info: "HANDSHAKE must use channel ID 0"}
		}
	}
	return nil
}
