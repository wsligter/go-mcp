# go-mcp

[![GitHub tag (latest
SemVer)](https://img.shields.io/github/v/tag/dstotijn/go-mcp?label=go%20module)](https://github.com/dstotijn/go-mcp/tags)
[![Go
Reference](https://pkg.go.dev/badge/github.com/dstotijn/go-mcp.svg)](https://pkg.go.dev/github.com/dstotijn/go-mcp)
[![GitHub](https://img.shields.io/github/license/dstotijn/go-mcp)](LICENSE)
[![Go Report
Card](https://goreportcard.com/badge/github.com/dstotijn/go-mcp)](https://goreportcard.com/report/github.com/dstotijn/go-mcp)

Go library for implementing the [Model Context
Protocol](https://modelcontextprotocol.io/) (MCP).

## Features

- [x] Supports protocol revision [2024-11-05](https://spec.modelcontextprotocol.io/specification/2024-11-05/)
- [x] Server support
- [x] Client support
- [x] Type safe RPC handlers without reflection
- [x] Built-in validation of tool arguments

## Installation

```
go get github.com/dstotijn/go-mcp
```

## Usage

See [examples/server/main.go](/examples/server/main.go) for a detailed example
of a server implementation.

## Deployment

This repository includes Azure deployment configurations:

- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Complete Azure Container Apps deployment guide
- **[COPILOT_STUDIO_SETUP.md](COPILOT_STUDIO_SETUP.md)** - Microsoft Copilot Studio integration
- **[DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md)** - Docker deployment options (Container Apps, App Service, AKS)
- **[AZURE_DEPLOYMENT.md](AZURE_DEPLOYMENT.md)** - Azure Functions deployment (custom handlers)

### Quick Deploy to Azure

```bash
# Set variables
RESOURCE_GROUP="your-resource-group"
REGISTRY_NAME="yourregistry"
CONTAINER_APP_NAME="your-app-name"

# Build and deploy
az acr build --registry $REGISTRY_NAME --image mcp-server:copilot --file Dockerfile.copilot .
az containerapp update --name $CONTAINER_APP_NAME --resource-group $RESOURCE_GROUP --image $REGISTRY_NAME.azurecr.io/mcp-server:copilot
```

See [DEPLOYMENT.md](DEPLOYMENT.md) for complete instructions.

## License

[Apache License 2.0](/LICENSE)

© 2025 David Stotijn
