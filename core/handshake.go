package core

import (
	"fmt"

	"github.com/golang/glog"
)

// HandshakeMsg holds a handshake message data payload
type HandshakeMsg struct {
	C ChanID
	S SwarmID
	// TODO: swarm SwarmMetadata
	// TODO: peer capabilities
}

// StartHandshake sends a starting handshake message to the remote peer on swarm sid
// onReady function is called asynchronously when the handshake is complete.
func (p *Ppspp) StartHandshake(remote PeerID, sid SwarmID, onReady func(PeerID)) error {
	p.lock()
	defer p.unlock()

	glog.V(1).Infof("%v starting handshake with %v", p.id, remote)

	ours := p.chooseOutChan()
	// their channel is 0 until they reply with a handshake
	p.addChan(ours, sid, 0, Begin, remote)
	p.chans[ours].state = WaitHandshake
	p.chans[ours].onReady = onReady
	return p.sendReqHandshake(ours, sid)
}

func (p *Ppspp) handleHandshake(cid ChanID, m Msg, remote PeerID) error {
	glog.V(1).Infof("%v handling handshake from %v", p.id, remote)
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
		glog.V(3).Infof("%v moving to ready state", p.id)
		if p.chans[newCID].onReady != nil {
			go p.chans[newCID].onReady(remote)
		}
		p.sendReplyHandshake(newCID, h.C, h.S)
	} else {
		c := p.chans[cid]
		switch c.state {
		case Begin:
			return MsgError{c: cid, m: m, info: "starting handshake must use channel ID 0"}
		case WaitHandshake:
			return p.handleReplyHandshake(h, cid)
		case Ready:
			glog.V(3).Infof("%v in ready state", p.id)
			if h.C == 0 {
				glog.V(3).Infof("%v received closing handshake", p.id)
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
	glog.V(3).Infof("%v sending request handshake", p.id)
	return p.sendHandshake(ours, 0, sid)
}

func (p *Ppspp) sendReplyHandshake(ours ChanID, theirs ChanID, sid SwarmID) error {
	glog.V(3).Infof("%v sending reply handshake ours=%v, theirs=%v", p.id, ours, theirs)
	return p.sendHandshake(ours, theirs, sid)
}

// SendClosingHandshake sends a closing handshake message to the remote peer on swarm sid
func (p *Ppspp) SendClosingHandshake(remote PeerID, sid SwarmID) error {
	p.lock()
	defer p.unlock()

	// get chanID from PeerID and SwarmID
	c := p.swarms[sid].chans[remote]

	glog.V(3).Infof("%v sending closing handshake on sid=%v c=%v to %v", p.id, sid, c, remote)
	// handshake with c=0 will signal a close handshake
	h := HandshakeMsg{C: 0, S: sid}
	m := Msg{Op: Handshake, Data: h}
	d := Datagram{ChanID: p.chans[c].theirs, Msgs: []Msg{m}}
	glog.V(3).Infof("%v sending datagram for closing handshake", p.id)
	err := p.sendDatagram(d, c)
	if err != nil {
		return fmt.Errorf("sendClosingHandshake: %v", err)
	}
	return p.closeChannel(c)
}

func (p *Ppspp) sendHandshake(ours ChanID, theirs ChanID, sid SwarmID) error {
	glog.V(3).Infof("%v sending handshake ours=%v, theirs=%v", p.id, ours, theirs)
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
	glog.V(3).Infof("%v in waitHandshake state", p.id)
	if h.C == 0 {
		glog.V(3).Infof("%v received closing handshake", p.id)
		p.closeChannel(cid)
	} else {
		c.theirs = h.C
		glog.V(3).Infof("%v moving to ready state", p.id)
		c.state = Ready
		if c.onReady != nil {
			go c.onReady(c.remote)
		}
	}

	return nil
}
