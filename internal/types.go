package internal

import (
	"encoding/json"
	"time"
)

type VideoChunk struct {
	Data      []byte    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	ChunkID   uint64    `json:"chunk_id"`
	Size      int       `json:"size"`
}

type StreamStats struct {
	ChunksSent     uint64
	ChunksReceived uint64
	DroppedChunks  uint64
	Bitrate        float64
	RTT            time.Duration
}

func (c *VideoChunk) Serialize() []byte {
	data, _ := json.Marshal(c)
	return data
}

func DeserializeChunk(data []byte) (*VideoChunk, error) {
	chunk := &VideoChunk{}
	err := json.Unmarshal(data, chunk)
	return chunk, err
}
