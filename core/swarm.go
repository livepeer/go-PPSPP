package core

import (
	"container/list"

	"fmt"

	"github.com/golang/glog"
)

// SwarmMetadata stores the metadata of a Swarm
// See: https://tools.ietf.org/html/rfc7574#section-3.1
type SwarmMetadata struct {
	ID        SwarmID
	ChunkSize int
	// TODO: chunk addressing method
	// TODO: content integrity protection method
	// TODO: Merkle hash tree function (if applicable)
}

// Swarm tracks info related to a swarm
type Swarm struct {
	// chans is a peer ID -> channel ID map for this swarm
	// it does not include this peer, because this peer does not have a local channel ID
	chans map[PeerID]ChanID
	// TODO: other swarm metadata stored here

	// chunkstore tracks the chunks that are locally stored for this swarm
	localChunks map[ChunkID]*Chunk

	// haves maps ChunkID to a list of peers that have that chunk (peers tracked by peer ID)
	remoteHaves map[ChunkID]*list.List

	metadata SwarmMetadata
}

// NewSwarm creates a new Swarm
func NewSwarm(metadata SwarmMetadata) *Swarm {
	chans := make(map[PeerID]ChanID)

	localChunks := make(map[ChunkID]*Chunk)

	remoteHaves := make(map[ChunkID]*list.List)

	return &Swarm{chans: chans, localChunks: localChunks, remoteHaves: remoteHaves, metadata: metadata}
}

//
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

func (s *Swarm) AddLocalChunks(start ChunkID, end ChunkID, data []byte) error {
	chunkSize := s.metadata.ChunkSize
	for i := start; i <= end; i++ {
		c := newChunk(i, chunkSize)
		dstart := (int(i) - int(start)) * chunkSize
		dend := dstart + chunkSize
		n := copy(c.B, data[dstart:dend])
		if n != chunkSize {
			return fmt.Errorf("AddLocalChunks bad copy")
		}
		s.localChunks[i] = c
	}
	return nil
}

func (s *Swarm) LocalChunks() map[ChunkID]*Chunk {
	return s.localChunks
}

// // LocalContent returns a buffer with all contiguous chunks available starting at id
// func (s *Swarm) LocalContent(id ChunkID) (*bytes.Buffer, error) {
// 	_, ok := s.localChunks[id]
// 	if !ok {
// 		return nil, fmt.Errorf("LocalContent could not find %v", id)
// 	}
// 	return nil, fmt.Errorf("LocalContent TODO")
// }

func (s *Swarm) WantChunk(id ChunkID) bool {
	// Simple implementation for now... if it's not in localChunks, we want it.
	_, ok := s.localChunks[id]
	return !ok
}

func (s *Swarm) DataFromLocalChunks(start ChunkID, end ChunkID) ([]byte, error) {
	chunkSize := s.metadata.ChunkSize
	n := int(end) - int(start) + 1
	if n <= 0 {
		return nil, fmt.Errorf("DataFromLocalChunks bad range (%d, %d)", start, end)
	}
	b := make([]byte, n*chunkSize)
	for i := start; i <= end; i++ {
		c, ok := s.localChunks[i]
		if !ok {
			return b, fmt.Errorf("DataFromLocalChunks could not find local chunk %d", i)
		}
		bstart := (int(i) - int(start)) * chunkSize
		bend := bstart + chunkSize
		bn := copy(b[bstart:bend], c.B)
		// bn, err := c.B.Read(b[bstart:bend])
		if bn != chunkSize {
			return b, fmt.Errorf("DataFromLocalChunks bad read from local chunk %d (read %d bytes, chunksize=%d", i, bn, chunkSize)
		}
		// if err != nil {
		// 	return b, err
		// }
	}
	return b, nil
}

func (s *Swarm) ChunkSize() int {
	return s.metadata.ChunkSize
}
