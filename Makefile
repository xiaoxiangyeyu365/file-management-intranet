BINARY := cloudbox

.PHONY: build run clean test

build:
	go build -o $(BINARY) ./cmd/server

run:
	go run ./cmd/server

clean:
	rm -f $(BINARY)

test:
	go test ./... -v
