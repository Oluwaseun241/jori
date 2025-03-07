package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/Oluwaseun241/jori/internal"
)

func main() {
	mode := flag.String("mode", "server", "server or peer")
	videoFile := flag.String("video", "", "path to video file(for peer mode)")
	peerID := flag.String("peer-id", "", "peer ID (for peer mode)")
	flag.Parse()

	switch *mode {
	case "server":
		server := internal.NewSignalingServer()
		http.HandleFunc("/ws", server.HandleWebSocket)
		fmt.Println("Signaling server running on :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))

	case "peer":
		if *videoFile == "" || *peerID == "" {
			log.Fatal("Video file and peer ID required for peer mode")
		}
		config := internal.StreamConfig{
			BufferSize:    1024 * 1024, // 1mb
			ChunkSize:     64 * 1024,
			MaxBitrate:    5000000, // 5Mbps
			MinBitrate:    500000,  // 50Kbps
			TargetLatency: 100,     // 100ms
		}

		peer, err := internal.NewPeer(config)
		if err != nil {
			log.Fatal(err)
		}

		if err := peer.SendVideo(*videoFile); err != nil {
			log.Fatal(err)
		}
	}
}
