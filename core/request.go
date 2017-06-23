package core

import "github.com/golang/glog"
import "fmt"

// RequestMsg holds a have message data payload
type RequestMsg struct {
	// TODO: start chunk / end chunk
	Start ChunkID
	End   ChunkID
}

// TODO: SendBatchRequests - like SendRequest but batches multiple requests into a single datagram

func (p *Peer) SendRequest(start ChunkID, end ChunkID, remote PeerID, sid SwarmID) error {
	glog.Infof("%v SendReq Chunk %v-%v, to %v, on %v", p.ID(), start, end, remote, sid)
	swarm, ok1 := p.swarms[sid]
	if !ok1 {
		return fmt.Errorf("SendRequest could not find %v", sid)
	}
	ours, ok2 := swarm.chans[remote]
	if !ok2 {
		return fmt.Errorf("SendRequest could not find channel for %v on %v", remote, sid)
	}
	c, ok3 := p.chans[ours]
	if !ok3 {
		return fmt.Errorf("SendRequest could not find channel %v", ours)
	}
	h := RequestMsg{Start: start, End: end}
	m := Msg{Op: Request, Data: h}
	d := Datagram{ChanID: c.theirs, Msgs: []Msg{m}}
	return p.sendDatagram(d, ours)
}

func (p *Peer) handleRequest(cid ChanID, m Msg, remote PeerID) error {
	glog.Infof("%v handleRequest from %v", p.ID(), remote)
	c, ok1 := p.chans[cid]
	if !ok1 {
		return fmt.Errorf("handleRequest could not find chan %v", cid)
	}
	sid := c.sw
	_, ok2 := p.swarms[sid]
	if !ok2 {
		return fmt.Errorf("handleRequest could not find %v", sid)
	}
	h, ok3 := m.Data.(RequestMsg)
	if !ok3 {
		return MsgError{c: cid, m: m, info: "could not convert to RequestMsg"}
	}
	glog.Infof("%v requested chunk %v-%v on %v", remote, h.Start, h.End, sid)
	return nil
}
