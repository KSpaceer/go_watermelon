


build:
	go build 

test:
	go test -cpu 1,4 -race ./...

fmt:
	go fmt ./...
