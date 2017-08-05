package core

import (
	"fmt"

	"github.com/golang/glog"
)

// HaveMsg holds a have message data payload
type HaveMsg struct {
	// TODO: start chunk / end chunk
	Start ChunkID
	End   ChunkID
}

// TODO: SendBatchHaves - like SendHave but batches multiple haves into a single datagram

// SendHave sends a have message for the chunk range to the remote peer on the Swarm
func (p *Ppspp) SendHave(start ChunkID, end ChunkID, remote PeerID, sid SwarmID) error {
	p.lock()
	defer p.unlock()

	return p.sendHave(start, end, remote, sid)
}
func (p *Ppspp) sendHave(start ChunkID, end ChunkID, remote PeerID, sid SwarmID) error {
	glog.V(1).Infof("%v sendHave Chunks %d-%d, to %v, on %v", p.id, start, end, remote, sid)
	swarm, ok1 := p.swarms[sid]
	if !ok1 {
		return fmt.Errorf("sendHave could not find %v", sid)
	}
	ours, ok2 := swarm.chans[remote]
	if !ok2 {
		return fmt.Errorf("sendHave could not find channel for %v on %v", remote, sid)
	}
	c, ok3 := p.chans[ours]
	if !ok3 {
		return fmt.Errorf("sendHave could not find channel %v", ours)
	}
	h := HaveMsg{Start: start, End: end}
	m := Msg{Op: Have, Data: h}
	d := Datagram{ChanID: c.theirs, Msgs: []Msg{m}}
	glog.V(3).Infof("%v sendHave ChanID=%d", p.id, c.theirs)
	return p.sendDatagram(d, ours)
}

func (p *Ppspp) handleHave(cid ChanID, m Msg, remote PeerID) error {
	glog.V(3).Infof("%v handleHave from %v", p.id, remote)
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
	glog.V(3).Infof("%v handleHave %d-%d from %v", p.id, h.Start, h.End, remote)
	return p.requestWantedChunksInRange(h.Start, h.End, remote, sid, s)
}

func (p *Ppspp) requestWantedChunksInRange(start ChunkID, end ChunkID, remote PeerID, sid SwarmID, s *Swarm) error {
	var startRange ChunkID
	var endRange ChunkID
	wantedRange := false
	for i := start; i <= end; i++ {
		s.AddRemoteHave(i, remote)
		if s.WantChunk(i) {
			endRange = i
			if !wantedRange {
				wantedRange = true
				startRange = i
			}
		} else {
			if wantedRange {
				wantedRange = false
				err := p.sendRequest(startRange, endRange, remote, sid)
				if err != nil {
					return err
				}
			}
		}
	}
	// If we left the for loop in a wantedRange, send the request for it
	if wantedRange {
		err := p.sendRequest(startRange, endRange, remote, sid)
		if err != nil {
			return err
		}
	}
	return nil
}
