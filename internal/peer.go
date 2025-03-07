package internal

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
)

type PeerConnection struct {
	connection  *webrtc.PeerConnection
	dataChannel *webrtc.DataChannel
	stats       *StreamStats
	buffer      map[uint64]*VideoChunk
	bufferMu    sync.Mutex
	config      StreamConfig
	lastRTT     time.Duration
	rttMu       sync.RWMutex
}

type StreamConfig struct {
	BufferSize    int           `json:"buffer_size"`
	ChunkSize     int           `json:"chunk_size"`
	MaxBitrate    int           `json:"max_bitrate"`
	MinBitrate    int           `json:"min_bitrate"`
	TargetLatency time.Duration `json:"target_latency"`
}

func NewPeer(config StreamConfig) (*PeerConnection, error) {
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	peer := &PeerConnection{
		connection: pc,
		stats:      &StreamStats{},
		buffer:     make(map[uint64]*VideoChunk),
		config:     config,
	}

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		if state == webrtc.ICEConnectionStateConnected {
			go peer.monitorRTT()
		}
	})

	return peer, nil
}

func (p *PeerConnection) monitorRTT() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := p.connection.GetStats()
		for _, stat := range stats {
			if s, ok := stat.(webrtc.ICECandidatePairStats); ok && s.State == "succeeded" {
				p.rttMu.Lock()
				p.lastRTT = time.Duration(s.CurrentRoundTripTime * float64(time.Second))
				p.rttMu.Unlock()
				break
			}
		}
	}
}

func (p *PeerConnection) getRTT() time.Duration {
	p.rttMu.RLock()
	defer p.rttMu.RUnlock()
	return p.lastRTT
}

func (p *PeerConnection) SendVideo(videoFile string) error {
	file, err := os.Open(videoFile)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, p.config.ChunkSize)
	ticker := time.NewTicker(time.Millisecond * 50)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			n, err := file.Read(buffer)
			if err != nil {
				return err
			}

			chunk := &VideoChunk{
				Data:      buffer[:n],
				Timestamp: time.Now(),
				ChunkID:   p.stats.ChunksSent,
				Size:      n,
			}

			// Adapt chunk size based on network conditions
			rtt := p.getRTT()
			p.stats.RTT = rtt

			if rtt > 100*time.Millisecond {
				p.config.ChunkSize = max(p.config.ChunkSize/2, p.config.MinBitrate)
			} else {
				p.config.ChunkSize = min(p.config.ChunkSize*2, p.config.MaxBitrate)
			}

			if err := p.dataChannel.Send(chunk.Serialize()); err != nil {
				log.Printf("Error sending chunk: %v", err)
				continue
			}

			p.stats.ChunksSent++
		}
	}
}

func (p *PeerConnection) ReceiveVideo(outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return err
	}
	defer file.Close()

	p.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		chunk, err := DeserializeChunk(msg.Data)
		if err != nil {
			log.Printf("Error deserializing chunk: %v", err)
			return
		}
		p.bufferMu.Lock()
		p.buffer[chunk.ChunkID] = chunk
		p.bufferMu.Unlock()

		p.stats.ChunksReceived++

		p.writeBufferedChunks(file)
	})
	return nil
}

func (p *PeerConnection) writeBufferedChunks(file *os.File) {
	p.bufferMu.Lock()
	defer p.bufferMu.Unlock()

	expectedID := p.stats.ChunksReceived - uint64(len(p.buffer))
	for {
		chunk, exists := p.buffer[expectedID]
		if !exists {
			break
		}

		file.Write(chunk.Data)
		delete(p.buffer, expectedID)
		expectedID++
	}
}

func (p *PeerConnection) SetupDataChannel() error {
	var err error
	p.dataChannel, err = p.connection.CreateDataChannel("video-stream", &webrtc.DataChannelInit{
		Ordered:        new(bool),   // true
		MaxRetransmits: new(uint16), // 0
	})
	if err != nil {
		return fmt.Errorf("failed to create data channel: %v", err)
	}

	p.dataChannel.OnOpen(func() {
		log.Println("Data channel opened")
	})

	p.dataChannel.OnClose(func() {
		log.Println("Data channel closed")
	})

	p.dataChannel.OnError(func(err error) {
		log.Printf("Data channel error: %v", err)
	})

	return nil
}
