package core

import "github.com/golang/glog"

func (p *Peer) SendHave(id ChunkID, remote PeerID, s SwarmID) {
	glog.Infof("SendHave Chunk %v, to %v, on %v", id, remote, s)
}
