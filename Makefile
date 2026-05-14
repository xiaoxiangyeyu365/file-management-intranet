BINARY := cloudbox
WEB_DIR := web
STATIC_DIR := cmd/server/static

.PHONY: build run clean test build-frontend

build: build-frontend
	go build -o $(BINARY) ./cmd/server

build-frontend:
	cd $(WEB_DIR) && npm run build

run:
	go run ./cmd/server

clean:
	rm -f $(BINARY)
	rm -rf $(STATIC_DIR)

test:
	go test ./... -v

# Development: run backend only (frontend served by Vite dev server)
dev:
	go run ./cmd/server