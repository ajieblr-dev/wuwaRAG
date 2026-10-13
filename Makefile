.PHONY: all build test run ingest dry-run docker-up docker-down clean

# Build binaries into bin/
build:
	@mkdir -p bin
	go build -o bin/api ./cmd/api
	go build -o bin/ingest ./cmd/ingest
	@echo "Build complete: bin/api, bin/ingest"

# Run all test suites
test:
	go test -v ./internal/...

# Run API server locally
run: build
	./bin/api

# Gather raw data from MediaWiki into data/raw/
gather: build
	./bin/ingest --pages "Simulacrum_of_the_Heart,Hsin,Suoming,Mengzhou,Xuanfang_Hold" --gather-only

# Gather all main quests (Prologue through Chapter IV) to data/raw/
gather-all: build
	./bin/ingest --all-main-quests --gather-only

# Run wiki ingestion CLI (sample pages)
ingest: build
	./bin/ingest --pages "Simulacrum_of_the_Heart,Hsin,Suoming,Mengzhou,Xuanfang_Hold"

# Ingest all main quests (Prologue through Chapter IV) into Qdrant
ingest-all: build
	./bin/ingest --all-main-quests

# Test parsing and preview chunks without writing to Qdrant
dry-run: build
	./bin/ingest --pages "Simulacrum_of_the_Heart,Hsin,Suoming,Mengzhou,Xuanfang_Hold" --dry-run

# Dry-run parse and preview for all main quests
dry-run-all: build
	./bin/ingest --all-main-quests --dry-run

# Start containers via Docker Compose / Podman Compose
docker-up:
	cd deploy && docker compose up -d || podman compose up -d

# Stop containers
docker-down:
	cd deploy && docker compose down || podman compose down

# Clean build artifacts
clean:
	rm -rf bin/
