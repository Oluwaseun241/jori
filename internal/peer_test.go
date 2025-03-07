package internal

import (
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/stretchr/testify/assert"
)

func TestNewPair(t *testing.T) {
	config := StreamConfig{
		BufferSize:    1024 * 1024,
		ChunkSize:     64 * 1024,
		MaxBitrate:    5000000,
		MinBitrate:    500000,
		TargetLatency: 100,
	}

	peer, err := NewPeer(config)
	assert.NoError(t, err)
	assert.NotNil(t, peer)
	assert.NotNil(t, peer.connection)
	assert.NotNil(t, peer.buffer)
	assert.Equal(t, config, peer.config)

}

func TestVideoChunkSerialization(t *testing.T) {
	chunk := &VideoChunk{
		Data:      []byte("test data"),
		Timestamp: time.Now(),
		ChunkID:   1,
		Size:      9,
	}

	serialized := chunk.Serialize()
	assert.NotEmpty(t, serialized)

	deserialized, err := DeserializeChunk(serialized)
	assert.NoError(t, err)
	assert.Equal(t, chunk.ChunkID, deserialized.ChunkID)
	assert.Equal(t, chunk.Size, deserialized.Size)
	assert.Equal(t, chunk.Data, deserialized.Data)

}

func TestPeerVideoTransfer(t *testing.T) {
	inputFile := "test_input.mp4"
	outputFile := "test_output.mp4"
	
	testData := []byte("test video content")
	err := os.WriteFile(inputFile, testData, 0644)
	assert.NoError(t, err)
	defer os.Remove(inputFile)
	defer os.Remove(outputFile)

	config := StreamConfig{
		BufferSize:    1024,
		ChunkSize:     256,
		MaxBitrate:    1000000,
		MinBitrate:    100000,
		TargetLatency: 100,
	}

	sender, err := NewPeer(config)
	assert.NoError(t, err)

	receiver, err := NewPeer(config)
	assert.NoError(t, err)

	// Set up data channels
	var wg sync.WaitGroup
	wg.Add(2)

	senderDC, err := sender.connection.CreateDataChannel("video-stream", nil)
	assert.NoError(t, err)
	sender.dataChannel = senderDC

	// Create channels to collect received data
	receivedData := make([]byte, 0)
	receivedMu := sync.Mutex{}

	// Handle ICE candidates
	sender.connection.OnICECandidate(func(ice *webrtc.ICECandidate) {
		if ice != nil {
			err := receiver.connection.AddICECandidate(ice.ToJSON())
			assert.NoError(t, err)
		}
	})

	receiver.connection.OnICECandidate(func(ice *webrtc.ICECandidate) {
		if ice != nil {
			err := sender.connection.AddICECandidate(ice.ToJSON())
			assert.NoError(t, err)
		}
	})

	receiver.connection.OnDataChannel(func(dc *webrtc.DataChannel) {
		receiver.dataChannel = dc
		dc.OnOpen(func() {
			wg.Done()
		})

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			chunk, err := DeserializeChunk(msg.Data)
			if err != nil {
				t.Errorf("Failed to deserialize chunk: %v", err)
				return
			}
			
			receivedMu.Lock()
			receivedData = append(receivedData, chunk.Data...)
			receivedMu.Unlock()
		})
	})

	senderDC.OnOpen(func() {
		wg.Done()
	})

	offer, err := sender.connection.CreateOffer(nil)
	assert.NoError(t, err)

	err = sender.connection.SetLocalDescription(offer)
	assert.NoError(t, err)

	err = receiver.connection.SetRemoteDescription(offer)
	assert.NoError(t, err)

	answer, err := receiver.connection.CreateAnswer(nil)
	assert.NoError(t, err)

	err = receiver.connection.SetLocalDescription(answer)
	assert.NoError(t, err)

	err = sender.connection.SetRemoteDescription(answer)
	assert.NoError(t, err)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for data channels to be ready")
	}

	// Start transfer
	transferDone := make(chan struct{})
	go func() {
		err := sender.SendVideo(inputFile)
		if err != nil && err != io.EOF {
			t.Errorf("Send error: %v", err)
		}
		close(transferDone)
	}()

	// Wait for transfer to complete with timeout
	select {
	case <-transferDone:
		// Transfer completed
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for transfer")
	}

	// Verify received data
	receivedMu.Lock()
	assert.Equal(t, testData, receivedData)
	receivedMu.Unlock()
}
