package core

import (
	"bytes"
)

// ChunkID identifies a chunk of content
type ChunkID uint32

// Chunk represents a PPSPP chunk of content
type Chunk struct {
	//io.Reader{}
	ID ChunkID
	B  *bytes.Buffer
}
