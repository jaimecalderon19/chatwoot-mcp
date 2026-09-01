.PHONY: build test lint run docker

BINARY := chatwoot-mcp

build:
	go build -o bin/$(BINARY) ./cmd/chatwoot-mcp

test:
	go test ./...

lint:
	golangci-lint run ./...

run:
	APP_ENV=development go run ./cmd/chatwoot-mcp

docker:
	docker build -t $(BINARY) .
