package core

import (
	"fmt"
	"log"

	peer "github.com/libp2p/go-libp2p-peer"
)

func (p *Peer) startHandshake(remote peer.ID, sid SwarmID) error {
	log.Printf("%v starting handshake", p.id())

	ours := p.chooseOurID()
	// their channel is 0 until they reply with a handshake
	p.addChan(ours, sid, 0, begin, remote)
	p.chans[ours].state = waitHandshake
	return p.sendReqHandshake(ours, sid)
}

func (p *Peer) handleHandshake(cid ChanID, m Msg, remote peer.ID) error {
	log.Printf("%v handling handshake", p.id())
	h, ok := m.Data.(HandshakeMsg)
	if !ok {
		return MsgError{c: cid, m: m, info: "could not convert to HANDSHAKE"}
	}

	// cid==0 means this is an incoming starting handshake
	if cid == 0 {
		if h.C < 1 {
			return MsgError{c: cid, m: m, info: "handshake cannot request channel ID 0"}
		}
		// need to create a new channel
		newCID := p.chooseOurID()
		p.addChan(newCID, h.S, h.C, ready, remote)
		log.Printf("%v moving to ready state", p.id())
		p.sendReplyHandshake(newCID, h.C, h.S)
	} else {
		c := p.chans[cid]
		switch c.state {
		case begin:
			return MsgError{c: cid, m: m, info: "starting handshake must use channel ID 0"}
		case waitHandshake:
			c := p.chans[cid]
			log.Println("in waitHandshake state")
			if h.C == 0 {
				log.Println("received closing handshake")
				p.closeChannel(cid)
			} else {
				c.theirs = h.C
				log.Printf("%v moving to ready state", p.id())
				c.state = ready
			}
		case ready:
			log.Println("in ready state")
			if h.C == 0 {
				log.Println("received closing handshake")
				p.closeChannel(cid)
			} else {
				return MsgError{c: cid, m: m, info: "got non-closing handshake while in ready state"}
			}
		default:
			return MsgError{c: cid, m: m, info: "bad channel state"}
		}
	}
	return nil
}

func (p *Peer) sendReqHandshake(ours ChanID, sid SwarmID) error {
	log.Printf("%v sending request handshake", p.id())
	return p.sendHandshake(ours, 0, sid)
}

func (p *Peer) sendReplyHandshake(ours ChanID, theirs ChanID, sid SwarmID) error {
	log.Printf("%v sending reply handshake", p.id())
	return p.sendHandshake(ours, theirs, sid)
}

func (p *Peer) sendClosingHandshake(remote peer.ID, sid SwarmID) error {
	// get chanID from peer.ID and SwarmID
	c := p.swarms[sid].chans[remote]

	log.Printf("%v sending closing handshake on sid=%v c=%v to %v", p.id(), sid, c, remote)
	// handshake with c=0 will signal a close handshake
	h := HandshakeMsg{C: 0}
	m := Msg{Op: Handshake, Data: h}
	d := Datagram{ChanID: p.chans[c].theirs, Msgs: []Msg{m}}
	log.Printf("%v sending datagram for closing handshake", p.id())
	err := p.sendDatagram(d, c)
	if err != nil {
		return fmt.Errorf("sendClosingHandshake: %v", err)
	}
	return p.closeChannel(c)
}

func (p *Peer) sendHandshake(ours ChanID, theirs ChanID, sid SwarmID) error {
	h := HandshakeMsg{C: ours, S: sid}
	m := Msg{Op: Handshake, Data: h}
	d := Datagram{ChanID: theirs, Msgs: []Msg{m}}
	return p.sendDatagram(d, ours)
}

func (p *Peer) chooseOurID() ChanID {
	// FIXME: see Issue #10
	return p.randomUnusedChanID()
}
