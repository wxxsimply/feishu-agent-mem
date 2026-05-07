.PHONY: all build build-mcp build-hooks test run clean docker-build docker-run

BINARY := bin/mem-service
MCP_BINARY := bin/mcp-server
HOOKS_BINARY := bin/openclaw-hooks
DOCKER_IMAGE := feishu-agent-mem:latest

all: build

build:
	@echo "Building feishu-agent-mem services..."
	@go build -o $(BINARY) ./cmd/mem-service/
	@go build -o $(MCP_BINARY) ./cmd/mcp-server/
	@go build -o $(HOOKS_BINARY) ./cmd/openclaw-hooks/
	@echo "Build complete!"

build-mcp:
	@echo "Building MCP server..."
	@go build -o $(MCP_BINARY) ./cmd/mcp-server/

build-hooks:
	@echo "Building OpenClaw hooks..."
	@go build -o $(HOOKS_BINARY) ./cmd/openclaw-hooks/

test:
	@go test ./internal/... -count=1 -timeout=60s

test-all:
	@go test ./test/... -v -count=1 -timeout=120s

test-benchmark:
	@go test ./test/benchmark/... -v -count=1 -timeout=300s

bench:
	@go test ./benchmarks/... -bench=. -benchtime=10s -count=1 -timeout=300s

run: build
	@./$(BINARY)

clean:
	@echo "Cleaning up..."
	@rm -f $(BINARY) $(MCP_BINARY) $(HOOKS_BINARY)
	@rm -rf data/

docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .

docker-run:
	@echo "Running Docker container..."
	@docker run -d \
		--name feishu-agent-mem \
		-p 37777:37777 \
		-v $(PWD)/data:/opt/feishu-agent-mem/data \
		-v $(PWD)/config:/opt/feishu-agent-mem/config \
		--env-file .env \
		$(DOCKER_IMAGE)

docker-stop:
	@docker stop feishu-agent-mem || true
	@docker rm feishu-agent-mem || true

fmt:
	@echo "Formatting code..."
	@go fmt ./...

vet:
	@echo "Vetting code..."
	@go vet ./...
