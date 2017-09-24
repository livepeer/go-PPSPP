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

	// map of outstanding requests (true if request outstanding, false otherwise)
	requests map[ChunkID]bool

	// haves maps ChunkID to a list of peers that have that chunk (peers tracked by peer ID)
	remoteHaves map[ChunkID]*list.List

	metadata SwarmMetadata
}

// NewSwarm creates a new Swarm
func NewSwarm(metadata SwarmMetadata) *Swarm {
	chans := make(map[PeerID]ChanID)

	localChunks := make(map[ChunkID]*Chunk)

	remoteHaves := make(map[ChunkID]*list.List)

	requests := make(map[ChunkID]bool)

	return &Swarm{chans: chans, localChunks: localChunks, remoteHaves: remoteHaves, metadata: metadata, requests: requests}
}

// AddRemoteHave tells this Swarm that the remote peer p has ChunkID c
func (s *Swarm) AddRemoteHave(c ChunkID, p PeerID) {
	_, ok := s.remoteHaves[c]
	if !ok {
		s.remoteHaves[c] = list.New()
	}
	s.remoteHaves[c].PushFront(p)
}

func (s *Swarm) AddRequest(c ChunkID) error {
	_, ok := s.requests[c]
	if ok {
		return fmt.Errorf("AddRequest adding request but one already exists")
	}
	s.requests[c] = true
	return nil
}

// Requested returns whether or not the request exists
func (s *Swarm) Requested(c ChunkID) bool {
	_, ok := s.requests[c]
	return ok
}

// RemoveRequest if the request exists, remove it
func (s *Swarm) RemoveRequest(c ChunkID) error {
	// _, ok := s.requests[c]
	// if !ok {
	// 	return fmt.Errorf("RemoveRequest removing request but none exists")
	// }
	// TODO: better to delete key?
	s.requests[c] = false
	return nil
}

// checkChunk returns whether the chunk has any errors (e.g. wrong size)
func (s *Swarm) checkChunk(c *Chunk) error {
	ref := s.ChunkSize()
	if size := len(c.B); ref != size {
		return fmt.Errorf("checkChunk got size=%d, should be %d", size, ref)
	}
	return nil
}

// AddLocalChunk stores the chunk locally
func (s *Swarm) AddLocalChunk(cid ChunkID, c *Chunk) error {
	if err := s.checkChunk(c); err != nil {
		return err
	}
	_, ok := s.localChunks[cid]
	if ok {
		glog.Warningf("addChunk overwriting existing chunk at id=%v", cid)
	}
	s.localChunks[cid] = c
	return nil
}

// AddLocalChunks stores a contiguous chunk range, with input data batched in one array
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

// LocalChunks returns the local chunks store for this Swarm
func (s *Swarm) LocalChunks() map[ChunkID]*Chunk {
	return s.localChunks
}

// WantChunk returns whether this Swarm wants the chunk locally
func (s *Swarm) WantChunk(id ChunkID) bool {
	// Simple implementation for now... if it's not in localChunks, and we don't have an outstanding request,
	// we want it.
	_, ok := s.localChunks[id]
	return (!ok && !s.requests[id])
}

// DataFromLocalChunks returns the data from the chunk range, packed into a single array
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
			return nil, fmt.Errorf("DataFromLocalChunks could not find local chunk %d", i)
		}
		bstart := (int(i) - int(start)) * chunkSize
		bend := bstart + chunkSize
		bn := copy(b[bstart:bend], c.B)
		if bn != chunkSize {
			return nil, fmt.Errorf("DataFromLocalChunks bad read from local chunk %d (read %d bytes, chunksize=%d", i, bn, chunkSize)
		}
	}
	return b, nil
}

// ChunkSize returns the chunk size for this Swarm
func (s *Swarm) ChunkSize() int {
	return s.metadata.ChunkSize
}

// ChanID returns the channel ID for the given peer, also returns bool indicating success of the lookup
func (s *Swarm) ChanID(id PeerID) (ChanID, bool) {
	c, ok := s.chans[id]
	return c, ok
}
