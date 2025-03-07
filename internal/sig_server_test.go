package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestSignalingServer(t *testing.T) {
	server := NewSignalingServer()
	ts := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	peer1, _, err := websocket.DefaultDialer.Dial(wsURL+"?peer_id=peer1", nil)
	assert.NoError(t, err)
	defer peer1.Close()

	peer2, _, err := websocket.DefaultDialer.Dial(wsURL+"?peer_id=peer2", nil)
	assert.NoError(t, err)
	defer peer2.Close()

	textMessage := SignalMessage{
		Type:    "offer",
		PeerID:  "peer2",
		Payload: json.RawMessage(`{"type": "offer", "sdp": "test sdp"}`),
	}

	messageJSON, err := json.Marshal(textMessage)
	assert.NoError(t, err)

	err = peer1.WriteMessage(websocket.TextMessage, messageJSON)
	assert.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, message, err := peer2.ReadMessage()
		assert.NoError(t, err)

		var receivedMsg SignalMessage
		err = json.Unmarshal(message, &receivedMsg)
		assert.NoError(t, err)
		assert.Equal(t, textMessage.Type, receivedMsg.Type)
		assert.Equal(t, textMessage.PeerID, receivedMsg.PeerID)

	}()

	select {
	case <-done:
	case <-time.After(time.Second * 2):
		t.Fatal("Timeout waiting for message relay")
	}
}

func TestSignalingServerInvalidPeerID(t *testing.T) {
	server := NewSignalingServer()
	ts := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	// Try to connect without peer_id
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Error("Expected error for missing peer_id, got nil")
	}
	if resp != nil && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestSignalingServerMessageBroadcast(t *testing.T) {
	server := NewSignalingServer()
	ts := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	peers := make([]*websocket.Conn, 3)
	for i := range peers {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?peer_id=peer"+string(rune('1'+i)), nil)
		assert.NoError(t, err)
		defer conn.Close()
		peers[i] = conn
	}

	testMessage := SignalMessage{
		Type:    "broadcast",
		PeerID:  "all",
		Payload: json.RawMessage(`{"message": "test broadcast"}`),
	}

	messageJSON, err := json.Marshal(testMessage)
	assert.NoError(t, err)

	err = peers[0].WriteMessage(websocket.TextMessage, messageJSON)
	assert.NoError(t, err)

	for i := 1; i < len(peers); i++ {
		_, message, err := peers[i].ReadMessage()
		assert.NoError(t, err)
		assert.NotNil(t, message)

		var receivedMsg SignalMessage
		err = json.Unmarshal(message, &receivedMsg)
		assert.NoError(t, err)
		assert.Equal(t, testMessage.Type, receivedMsg.Type)
	}
}
