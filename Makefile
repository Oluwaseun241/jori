run:
	go run main.go -mode server

peer:
	go run main.go -mode peer -peer-id sender -video path/to/video.mp4
	
