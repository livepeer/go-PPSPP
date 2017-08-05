package core

import (
	"fmt"
)

// HandshakeMsg holds a handshake message data payload
type HandshakeMsg struct {
	C ChanID
	S SwarmID
	// TODO: swarm SwarmMetadata
	// TODO: peer capabilities
}

// StartHandshake sends a starting handshake message to the remote peer on swarm sid
func (p *Ppspp) StartHandshake(remote PeerID, sid SwarmID) error {
	p.lock()
	defer p.unlock()

	p.infof(1, "starting handshake with %v", remote)

	ours := p.chooseOutChan()
	// their channel is 0 until they reply with a handshake
	p.addChan(ours, sid, 0, Begin, remote)
	p.chans[ours].state = WaitHandshake
	return p.sendReqHandshake(ours, sid)
}

func (p *Ppspp) handleHandshake(cid ChanID, m Msg, remote PeerID) error {
	p.infof(1, "handling handshake from %v", remote)
	h, ok := m.Data.(HandshakeMsg)
	if !ok {
		return MsgError{c: cid, m: m, info: "could not convert to Handshake"}
	}

	// cid==0 means this is an incoming starting handshake
	if cid == 0 {
		if h.C < 1 {
			return MsgError{c: cid, m: m, info: "handshake cannot request channel ID 0"}
		}

		// Check if we already have a channel for this remote peer in the swarm
		cid, ok = p.chanIDForSwarmAndPeer(h.S, remote)
		if ok {
			c := p.chans[cid]
			// if a channel already exists, and we're expecting a reply handshake, then we assume the remote
			// peer sent a handshake request before it received our request. Treat it as a reply.
			if c.state == WaitHandshake {
				return p.handleReplyHandshake(h, cid)
			}
			return fmt.Errorf("handshake error: received handshake request but a channel already exists for the remote peer")
		}
		// need to create a new channel
		newCID := p.chooseOutChan()
		if err := p.addChan(newCID, h.S, h.C, Ready, remote); err != nil {
			return err
		}
		p.infof(3, "moving to ready state")
		p.sendReplyHandshake(newCID, h.C, h.S)
	} else {
		c := p.chans[cid]
		switch c.state {
		case Begin:
			return MsgError{c: cid, m: m, info: "starting handshake must use channel ID 0"}
		case WaitHandshake:
			return p.handleReplyHandshake(h, cid)
		case Ready:
			p.info(3, "in ready state")
			if h.C == 0 {
				p.info(3, "received closing handshake")
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

func (p *Ppspp) sendReqHandshake(ours ChanID, sid SwarmID) error {
	p.infof(3, "sending request handshake")
	return p.sendHandshake(ours, 0, sid)
}

func (p *Ppspp) sendReplyHandshake(ours ChanID, theirs ChanID, sid SwarmID) error {
	p.infof(3, "sending reply handshake ours=%v, theirs=%v", ours, theirs)
	return p.sendHandshake(ours, theirs, sid)
}

// SendClosingHandshake sends a closing handshake message to the remote peer on swarm sid
func (p *Ppspp) SendClosingHandshake(remote PeerID, sid SwarmID) error {
	p.lock()
	defer p.unlock()

	// get chanID from PeerID and SwarmID
	c := p.swarms[sid].chans[remote]

	p.infof(3, "sending closing handshake on sid=%v c=%v to %v", sid, c, remote)
	// handshake with c=0 will signal a close handshake
	h := HandshakeMsg{C: 0, S: sid}
	m := Msg{Op: Handshake, Data: h}
	d := Datagram{ChanID: p.chans[c].theirs, Msgs: []Msg{m}}
	p.infof(3, "sending datagram for closing handshake")
	err := p.sendDatagram(d, c)
	if err != nil {
		return fmt.Errorf("sendClosingHandshake: %v", err)
	}
	return p.closeChannel(c)
}

func (p *Ppspp) sendHandshake(ours ChanID, theirs ChanID, sid SwarmID) error {
	p.infof(3, "sending handshake ours=%v, theirs=%v", ours, theirs)
	h := HandshakeMsg{C: ours, S: sid}
	m := Msg{Op: Handshake, Data: h}
	d := Datagram{ChanID: theirs, Msgs: []Msg{m}}
	return p.sendDatagram(d, ours)
}

func (p *Ppspp) chooseOutChan() ChanID {
	// FIXME: see Issue #10
	return p.randomUnusedChanID()
}

func (p *Ppspp) handleReplyHandshake(h HandshakeMsg, cid ChanID) error {
	c, ok := p.chans[cid]
	if !ok {
		return fmt.Errorf("handleReplyHandshake error: could not find channel")
	}
	p.info(3, "in waitHandshake state")
	if h.C == 0 {
		p.info(3, "received closing handshake")
		p.closeChannel(cid)
	} else {
		c.theirs = h.C
		p.infof(3, "moving to ready state")
		c.state = Ready
	}

	return nil
}
