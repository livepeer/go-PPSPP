package core

import (
	"bytes"
	"container/list"

	"fmt"

	"github.com/golang/glog"
)

type Swarm struct {
	// chans is a peer ID -> channel ID map for this swarm
	// it does not include this peer, because this peer does not have a local channel ID
	chans map[PeerID]ChanID
	// TODO: other swarm metadata stored here

	// chunkstore tracks the chunks that are locally stored for this swarm
	localChunks map[ChunkID]*Chunk

	// haves maps ChunkID to a list of peers that have that chunk (peers tracked by peer ID)
	remoteHaves map[ChunkID]*list.List
}

func NewSwarm() *Swarm {
	chans := make(map[PeerID]ChanID)

	localChunks := make(map[ChunkID]*Chunk)

	remoteHaves := make(map[ChunkID]*list.List)

	return &Swarm{chans: chans, localChunks: localChunks, remoteHaves: remoteHaves}
}

func (s *Swarm) AddRemoteHave(c ChunkID, p PeerID) {
	_, ok := s.remoteHaves[c]
	if !ok {
		s.remoteHaves[c] = list.New()
	}
	s.remoteHaves[c].PushFront(p)
}

func (s *Swarm) AddLocalChunk(cid ChunkID, c *Chunk) {
	_, ok := s.localChunks[cid]
	if ok {
		glog.Warningf("addChunk overwriting existing chunk at id=%v", cid)
	}
	s.localChunks[cid] = c
}

func (s *Swarm) LocalChunks() map[ChunkID]*Chunk {
	return s.localChunks
}

// LocalContent returns a buffer with all contiguous chunks available starting at id
func (s *Swarm) LocalContent(id ChunkID) (*bytes.Buffer, error) {
	_, ok := s.localChunks[id]
	if !ok {
		return nil, fmt.Errorf("LocalContent could not find %v", id)
	}
	return nil, fmt.Errorf("LocalContent TODO")
}
