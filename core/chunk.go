package core

// ChunkID identifies a chunk of content
type ChunkID uint32

// Chunk represents a PPSPP chunk of content
type Chunk struct {
	//io.Reader{}
	ID ChunkID
	B  []byte
}

func newChunk(id ChunkID, size int) *Chunk {
	var c Chunk
	c.B = make([]byte, size)
	c.ID = id
	return &c
}
