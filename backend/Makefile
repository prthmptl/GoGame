.PHONY: build test lint vet run-api run-worker docker dev-up dev-down

VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

build:
	go build -trimpath -ldflags "-X main.version=$(VERSION)" ./...

vet:
	go vet ./...

lint:
	gofmt -l . | tee /dev/stderr | (! read)

test:
	go test -race ./...

docker:
	docker build --build-arg VERSION=$(VERSION) -t gogame-backend:$(VERSION) .

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

dev-up:
	docker compose -f deploy/docker-compose.yml up -d

dev-down:
	docker compose -f deploy/docker-compose.yml down -v
