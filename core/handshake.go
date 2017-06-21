package core

import (
	"fmt"

	"github.com/golang/glog"
	libp2ppeer "github.com/libp2p/go-libp2p-peer"
)

func (p *Peer) startHandshake(remote libp2ppeer.ID, sid SwarmID) error {
	glog.Infof("%v starting handshake", p.id())

	ours := p.chooseOutChan()
	// their channel is 0 until they reply with a handshake
	p.addChan(ours, sid, 0, Begin, remote)
	p.chans[ours].state = WaitHandshake
	return p.sendReqHandshake(ours, sid)
}

func (p *Peer) handleHandshake(cid ChanID, m Msg, remote libp2ppeer.ID) error {
	glog.Infof("%v handling handshake", p.id())
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
		newCID := p.chooseOutChan()
		p.addChan(newCID, h.S, h.C, Ready, remote)
		glog.Infof("%v moving to ready state", p.id())
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
				glog.Infof("%v moving to ready state", p.id())
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

func (p *Peer) sendReqHandshake(ours ChanID, sid SwarmID) error {
	glog.Infof("%v sending request handshake", p.id())
	return p.sendHandshake(ours, 0, sid)
}

func (p *Peer) sendReplyHandshake(ours ChanID, theirs ChanID, sid SwarmID) error {
	glog.Infof("%v sending reply handshake", p.id())
	return p.sendHandshake(ours, theirs, sid)
}

func (p *Peer) sendClosingHandshake(remote libp2ppeer.ID, sid SwarmID) error {
	// get chanID from libp2ppeer.ID and SwarmID
	c := p.swarms[sid].chans[remote]

	glog.Infof("%v sending closing handshake on sid=%v c=%v to %v", p.id(), sid, c, remote)
	// handshake with c=0 will signal a close handshake
	h := HandshakeMsg{C: 0}
	m := Msg{Op: Handshake, Data: h}
	d := Datagram{ChanID: p.chans[c].theirs, Msgs: []Msg{m}}
	glog.Infof("%v sending datagram for closing handshake", p.id())
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

func (p *Peer) chooseOutChan() ChanID {
	// FIXME: see Issue #10
	return p.randomUnusedChanID()
}
