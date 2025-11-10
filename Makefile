.PHONY: build-azure clean

# Build the Azure Functions custom handler
build-azure:
	GOOS=linux GOARCH=amd64 go build -o handler cmd/azure-handler/main.go

# Clean build artifacts
clean:
	rm -f handler
