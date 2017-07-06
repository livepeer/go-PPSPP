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
func (p *Ppspp) StartHandshake(remote PeerID, sid SwarmID) error {
	glog.Infof("starting handshake with %v", remote)

	ours := p.chooseOutChan()
	// their channel is 0 until they reply with a handshake
	p.addChan(ours, sid, 0, Begin, remote)
	p.chans[ours].state = WaitHandshake
	return p.sendReqHandshake(ours, sid)
}

func (p *Ppspp) handleHandshake(cid ChanID, m Msg, remote PeerID) error {
	glog.Infof("handling handshake from %v", remote)
	h, ok := m.Data.(HandshakeMsg)
	if !ok {
		return MsgError{c: cid, m: m, info: "could not convert to Handshake"}
	}

	// cid==0 means this is an incoming starting handshake
	if cid == 0 {
		if h.C < 1 {
			return MsgError{c: cid, m: m, info: "handshake cannot request channel ID 0"}
		}
		// need to create a new channel
		newCID := p.chooseOutChan()
		p.addChan(newCID, h.S, h.C, Ready, remote)
		glog.Infof("moving to ready state")
		p.sendReplyHandshake(newCID, h.C, h.S)
	} else {
		c := p.chans[cid]
		switch c.state {
		case Begin:
			return MsgError{c: cid, m: m, info: "starting handshake must use channel ID 0"}
		case WaitHandshake:
			c := p.chans[cid]
			glog.Info("in waitHandshake state")
			if h.C == 0 {
				glog.Info("received closing handshake")
				p.closeChannel(cid)
			} else {
				c.theirs = h.C
				glog.Infof("moving to ready state")
				c.state = Ready
			}
		case Ready:
			glog.Info("in ready state")
			if h.C == 0 {
				glog.Info("received closing handshake")
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
	glog.Infof("sending request handshake")
	return p.sendHandshake(ours, 0, sid)
}

func (p *Ppspp) sendReplyHandshake(ours ChanID, theirs ChanID, sid SwarmID) error {
	glog.Infof("sending reply handshake ours=%v, theirs=%v", ours, theirs)
	return p.sendHandshake(ours, theirs, sid)
}

// SendClosingHandshake sends a closing handshake message to the remote peer on swarm sid
func (p *Ppspp) SendClosingHandshake(remote PeerID, sid SwarmID) error {
	// get chanID from PeerID and SwarmID
	c := p.swarms[sid].chans[remote]

	glog.Infof("sending closing handshake on sid=%v c=%v to %v", sid, c, remote)
	// handshake with c=0 will signal a close handshake
	h := HandshakeMsg{C: 0, S: sid}
	m := Msg{Op: Handshake, Data: h}
	d := Datagram{ChanID: p.chans[c].theirs, Msgs: []Msg{m}}
	glog.Infof("sending datagram for closing handshake")
	err := p.sendDatagram(d, c)
	if err != nil {
		return fmt.Errorf("sendClosingHandshake: %v", err)
	}
	return p.closeChannel(c)
}

func (p *Ppspp) sendHandshake(ours ChanID, theirs ChanID, sid SwarmID) error {
	glog.Infof("sending handshake ours=%v, theirs=%v", ours, theirs)
	h := HandshakeMsg{C: ours, S: sid}
	m := Msg{Op: Handshake, Data: h}
	d := Datagram{ChanID: theirs, Msgs: []Msg{m}}
	return p.sendDatagram(d, ours)
}

func (p *Ppspp) chooseOutChan() ChanID {
	// FIXME: see Issue #10
	return p.randomUnusedChanID()
}
