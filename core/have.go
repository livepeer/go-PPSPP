package core

import "github.com/golang/glog"
import "fmt"

// HaveMsg holds a have message data payload
type HaveMsg struct {
	C ChunkID
}

// TODO: SendBatchHaves - like SendHave but batches multiple haves into a single datagram

func (p *Peer) SendHave(id ChunkID, remote PeerID, sid SwarmID) error {
	glog.Infof("%v SendHave Chunk %v, to %v, on %v", p.ID(), id, remote, sid)

	swarm, ok1 := p.swarms[sid]
	if !ok1 {
		return fmt.Errorf("SendHave could not find %v", sid)
	}
	ours, ok2 := swarm.chans[remote]
	if !ok2 {
		return fmt.Errorf("SendHave could not find channel for %v on %v", remote, sid)
	}
	c, ok3 := p.chans[ours]
	if !ok3 {
		return fmt.Errorf("SendHave could not find channel %v", ours)
	}

	h := HaveMsg{C: id}
	m := Msg{Op: Have, Data: h}
	d := Datagram{ChanID: c.theirs, Msgs: []Msg{m}}
	return p.sendDatagram(d, ours)
}

func (p *Peer) handleHave(cid ChanID, m Msg, remote PeerID) error {
	glog.Infof("%v handleHave from %v", p.ID(), remote)
	c, ok1 := p.chans[cid]
	if !ok1 {
		return fmt.Errorf("handleHave could not find chan %v", cid)
	}
	sid := c.sw
	s, ok2 := p.swarms[sid]
	if !ok2 {
		return fmt.Errorf("handleHave could not find %v", sid)
	}

	h, ok3 := m.Data.(HaveMsg)
	if !ok3 {
		return MsgError{c: cid, m: m, info: "could not convert to Have"}
	}
	s.AddRemoteHave(h.C, remote)
	return nil
}
