package core

import (
	"errors"
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

type ChanID uint

type Opcode uint8

const (
	HANDSHAKE Opcode = 1
)

type Msg struct {
	op   Opcode
	data interface{}
}

type Handshake struct {
	c ChanID
	// TODO: swarm SwarmMetadata
	// TODO: peer capabilities
}

type Datagram struct {
	chanID ChanID
	msgs   []Msg
}

type ProtocolState uint

const (
	INIT           ProtocolState = 0
	WAIT_HANDSHAKE ProtocolState = 1 // waiting for ack of first handshake
	READY          ProtocolState = 2
)

type Chan struct {
	ours   ChanID // receiving channel id
	theirs ChanID // sending channel id
	state  ProtocolState
}

type Peer struct {
	chans map[ChanID](*Chan)
}

func NewPeer() *Peer {
	chans := make(map[ChanID](*Chan))
	return &Peer{chans: chans}
}

func (p *Peer) HandleDatagram(d Datagram) error {
	for _, msg := range d.msgs {
		err := p.handleMsg(p.chans[d.chanID], msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Peer) handleMsg(c *Chan, m Msg) error {
	switch m.op {
	case HANDSHAKE:
		return p.handleHandshake(c, m)
	}
	return nil
}

func (p *Peer) handleHandshake(c *Chan, m Msg) error {
	h := m.data.(Handshake)
	switch c.state {
	case INIT:
		if c.ours != 0 {
			return MsgError{c: c.ours, m: m, info: "HANDSHAKE must use channel ID 0"}
		}
		if h.c < 1 {
			return MsgError{c: c.ours, m: m, info: "HANDSHAKE cannot request channel ID 0"}
		}
		c.theirs = h.c
		p.sendReplyHandshake(c.theirs)
		c.state = READY
	case WAIT_HANDSHAKE:
		if h.c < 1 {
			return MsgError{c: c.ours, m: m, info: "HANDSHAKE cannot request channel ID 0"}
		}
		c.theirs = h.c
		c.state = READY
	default:
		return MsgError{c: c.ours, m: m, info: "bad channel state"}
	}
	return nil
}

func (p *Peer) setupChan(ours ChanID) error {
	if ours < 1 {
		return errors.New("cannot setup channel with ours<1")
	}
	p.chans[ours] = &Chan{ours: ours, state: INIT}
	return nil
}

func (p *Peer) startHandshake() ChanID {
	ours := chooseOurID()
	p.setupChan(ours)
	p.chans[ours].state = WAIT_HANDSHAKE
	p.sendReqHandshake(ours)
	return ours
}

func (p *Peer) sendReqHandshake(ours ChanID) {
	p.sendHandshake(ours, 0)
}

func (p *Peer) sendReplyHandshake(theirs ChanID) {
	ours := chooseOurID()
	p.sendHandshake(ours, theirs)
}

func (p *Peer) sendHandshake(ours ChanID, theirs ChanID) {
	h := Handshake{c: ours}
	// send on p.chans[theirs]
	fmt.Println(h)
}

func chooseOurID() ChanID {
	// TODO
	return 1
}
