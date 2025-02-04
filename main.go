package main

import (
	"fmt"
	"net/http"

	"github.com/Oluwaseun241/jori/internal"
)

func main() {
	http.HandleFunc("/ws", internal.HandleWebSocket)
	fmt.Println("Signaling server running on :8080")
	http.ListenAndServe(":8080", nil)
}
