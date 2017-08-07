package core

import "github.com/golang/glog"
import "fmt"

// DataMsg holds a data message data payload
type DataMsg struct {
	Start ChunkID
	End   ChunkID
	Data  []byte
}

// SendData sends the chunk range in a data message
func (p *Ppspp) SendData(start ChunkID, end ChunkID, remote PeerID, sid SwarmID) error {
	glog.Infof("SendData Chunks %d-%d, to %v, on %v", start, end, remote, sid)
	swarm, ok1 := p.swarms[sid]
	if !ok1 {
		return fmt.Errorf("SendData could not find %v", sid)
	}
	ours, ok2 := swarm.chans[remote]
	if !ok2 {
		return fmt.Errorf("SendData could not find channel for %v on %v", remote, sid)
	}
	c, ok3 := p.chans[ours]
	if !ok3 {
		return fmt.Errorf("SendData could not find channel %v", ours)
	}
	data, err := swarm.DataFromLocalChunks(start, end)
	if err != nil {
		return err
	}
	h := DataMsg{Start: start, End: end, Data: data}
	m := Msg{Op: Data, Data: h}
	d := Datagram{ChanID: c.theirs, Msgs: []Msg{m}}
	return p.sendDatagram(d, ours)
}

func (p *Ppspp) handleData(cid ChanID, m Msg, remote PeerID) error {
	glog.Infof("handleData from %v", remote)
	c, ok1 := p.chans[cid]
	if !ok1 {
		return fmt.Errorf("handleData could not find chan %v", cid)
	}
	sid := c.sw
	swarm, ok2 := p.swarms[sid]
	if !ok2 {
		return fmt.Errorf("handleData could not find %v", sid)
	}
	d, ok3 := m.Data.(DataMsg)
	if !ok3 {
		return MsgError{c: cid, m: m, info: "could not convert to DataMsg"}
	}
	glog.Infof("recvd data %d-%d from %v on %v", d.Start, d.End, remote, sid)
	// TODO: skipping integrity check
	if err := swarm.AddLocalChunks(d.Start, d.End, d.Data); err != nil {
		return err
	}
	// Invoke the data handler if we have one
	if swarm.dataHandler != nil {
		go swarm.dataHandler(d)
	}
	// Send haves to all peers in the swarm
	for r := range swarm.chans {
		if err := p.SendHave(d.Start, d.End, r, sid); err != nil {
			return err
		}
	}
	return nil
}
