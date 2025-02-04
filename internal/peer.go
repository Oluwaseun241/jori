package internal

import (
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

func main() {

}
