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

// SendRequest sends a request for the chunk range to the remote peer on the swarm
func (p *Ppspp) SendRequest(start ChunkID, end ChunkID, remote PeerID, sid SwarmID) error {
	glog.Infof("SendReq Chunk %v-%v, to %v, on %v", start, end, remote, sid)
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

func (p *Ppspp) handleRequest(cid ChanID, m Msg, remote PeerID) error {
	glog.Infof("handleRequest from %v", remote)
	c, ok1 := p.chans[cid]
	if !ok1 {
		return fmt.Errorf("handleRequest could not find chan %v", cid)
	}
	sid := c.sw
	swarm, ok2 := p.swarms[sid]
	if !ok2 {
		return fmt.Errorf("handleRequest could not find %v", sid)
	}
	r, ok3 := m.Data.(RequestMsg)
	if !ok3 {
		return MsgError{c: cid, m: m, info: "could not convert to RequestMsg"}
	}
	glog.Infof("%v requested chunk %v-%v on %v", remote, r.Start, r.End, sid)
	return p.sendLocalChunksInRange(r.Start, r.End, remote, sid, swarm)
}

// Send any chunks in range that we have locally
func (p *Ppspp) sendLocalChunksInRange(start ChunkID, end ChunkID, remote PeerID, sid SwarmID, s *Swarm) error {
	var startRange ChunkID
	var endRange ChunkID
	haveRange := false
	for i := start; i <= end; i++ {
		_, ok := s.localChunks[i]
		if ok {
			endRange = i
			if !haveRange {
				haveRange = true
				startRange = i
			}
		} else {
			if haveRange {
				haveRange = false
				err := p.SendData(startRange, endRange, remote, sid)
				if err != nil {
					return err
				}
			}
		}
	}
	// If we left the for loop in a haveRange, send the request for it
	if haveRange {
		err := p.SendData(startRange, endRange, remote, sid)
		if err != nil {
			return err
		}
	}
	return nil
}
