.PHONY: build test test-local test-ci test-up test-down

build:
	go build -o ./bin/dbtree ./cmd/dbtree/

# Main test target - starts containers, runs tests, stops containers
test: test-up
	@echo "Waiting for databases to be ready..."
	@sleep 10
	go test -v ./...
	$(MAKE) test-down

# For local development - keeps containers running
test-local: test-up
	@echo "Waiting for databases to be ready..."
	@sleep 5
	go test -v ./...
	@echo "Containers still running. Use 'make test-down' to stop them."

# For CI - assumes containers are already running
test-ci:
	@echo "Running tests (assuming databases are already up)..."
	go test -v ./...

# Start test databases
test-up:
	docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for health checks..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if docker-compose -f docker-compose.test.yml ps | grep -q "healthy"; then break; fi; \
		echo "Waiting for databases to be healthy ($$i/10)..."; \
		sleep 2; \
	done

# Stop test databases
test-down:
	docker-compose -f docker-compose.test.yml down -v
