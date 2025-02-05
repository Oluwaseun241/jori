package internal

import (
	"fmt"
	"os"
	"time"

	"github.com/pion/webrtc/v3"
)

func sendVideo(dataChannel *webrtc.DataChannel, videoFile string) error {
	file, err := os.Open(videoFile)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, 1024*64)
	for {
		n, err := file.Read(buffer)
		if err != nil {
			break
		}

		dataChannel.Send(buffer[:n])
		//simulate
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func receiveVideo(dataChannel *webrtc.DataChannel) {
	file, err := os.Create("output.mp4")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		file.Write(msg.Data)
	})
}

func main() {
	config := webrtc.Configuration{}
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	dataChannel, err := peerConnection.CreateDataChannel("video-stream", nil)
	if err != nil {
		panic(err)
	}

	dataChannel.OnOpen(func() {
		fmt.Println("Data channel opened")
		// stream video
		sendVideo(dataChannel, "sample.mp4")
	})

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		fmt.Println("Recieved video chunk of size:", len(msg.Data))
	})

	select {}
}
