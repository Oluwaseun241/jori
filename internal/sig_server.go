package internal

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var connections []*websocket.Conn

func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading: ", err)
		return
	}
	defer conn.Close()
	connections = append(connections, conn)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Read error:", err)
			return
		}

		// Broadcast message to all peer
		for _, c := range connections {
			if c != conn {
				c.WriteMessage(websocket.TextMessage, msg)
			}
		}
	}
}
