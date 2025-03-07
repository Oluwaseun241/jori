package internal

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type SignalingServer struct {
	upgrader    websocket.Upgrader
	connections map[string]*websocket.Conn
	mu          sync.RWMutex
}

type SignalMessage struct {
	Type    string          `json:"type"`
	PeerID  string          `json:"peer_id"`
	Payload json.RawMessage `json:"payload"`
}

func NewSignalingServer() *SignalingServer {
	return &SignalingServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		connections: make(map[string]*websocket.Conn),
	}
}

func (s *SignalingServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}
	peerID := r.URL.Query().Get("peer_id")
	if peerID == "" {
		conn.Close()
		return
	}

	s.mu.Lock()
	s.connections[peerID] = conn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.connections, peerID)
		s.mu.Unlock()
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			return
		}

		var signal SignalMessage
		if err := json.Unmarshal(msg, &signal); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		s.mu.RLock()
		targetConn, exists := s.connections[signal.PeerID]
		s.mu.RUnlock()

		if exists {
			if err := targetConn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("Error forwarding message: %v", err)
			}
		}
	}
}
