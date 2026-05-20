BINARY := cloudbox
WEB_DIR := web
STATIC_DIR := cmd/server/static

.PHONY: build run clean test build-frontend docker-build docker-up docker-down

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

# Docker commands
docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

docker-restart: docker-down docker-up

# Full cleanup including volumes
docker-clean: docker-down
	docker compose down -v --remove-orphans
	docker rmi cloudbox-cloudbox 2>/dev/null || true