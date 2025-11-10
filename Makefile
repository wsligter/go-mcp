.PHONY: build-azure build-docker docker-run docker-test clean

# Build the Azure Functions custom handler
build-azure:
	GOOS=linux GOARCH=amd64 go build -o handler cmd/azure-handler/main.go

# Build Docker image
build-docker:
	docker build -t mcp-server:latest .

# Run Docker container locally
docker-run:
	docker run -p 8080:8080 mcp-server:latest

# Test Docker container
docker-test:
	docker run -p 8080:8080 --rm mcp-server:latest

# Clean build artifacts
clean:
	rm -f handler
	docker rmi mcp-server:latest 2>/dev/null || true
