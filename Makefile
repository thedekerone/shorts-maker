.PHONY: build clean run

build:
	GOARCH=amd64 CGO_ENABLED=1 go build -o video-processor cmd/server/main.go

run: build
	./video-processor

clean:
	rm -rf temp_frames
	rm -f output_with_subs.mp4

test:
	GOARCH=amd64 CGO_ENABLED=1 go test ./...
