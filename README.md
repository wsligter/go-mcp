# CBS MCP Server for Microsoft Copilot Studio

MCP (Model Context Protocol) server providing access to CBS (Statistics Netherlands) Open Data API, deployed to Azure Container Apps for Microsoft Copilot Studio integration.

## Features

✅ **7 CBS Open Data Tools**
- `get_catalogs` - List available data catalogs
- `query_datasets` - Search datasets with OData filters
- `get_dimensions` - Explore dataset structure
- `query_observations` - Query statistical data with OData
- `get_observations` - Retrieve observations with simple filters
- `get_metadata` - Get OData metadata document
- `get_dimension_values` - Get all values for a dimension

✅ **Copilot Studio Optimized**
- Comprehensive instructions for LLM understanding
- Dutch terminology translations
- Step-by-step workflow guidance
- Example queries and filter syntax

✅ **Production Ready**
- Deployed on Azure Container Apps
- Streamable HTTP transport (MCP 2024-11-05)
- Stateless design for horizontal scaling
- Full MCP spec compliance

## Quick Start

### Use with Copilot Studio

1. **Add MCP Server** in Copilot Studio:
   - URL: `https://roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io/mcp`
   - Transport: Streamable
   - Authentication: None

2. **Test with**: "What was the population of the Netherlands in 2022?"

### Local Development

```bash
# Build
go build -o mcp-server ./cmd/copilot-handler/

# Run
./mcp-server
# Server starts on http://localhost:8080
```

### Test Locally

```bash
# List tools
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":1}'

# Query CBS catalogs
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_catalogs","arguments":{}},"id":2}'
```

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for Azure Container Apps deployment instructions.

## Project Structure

```
go-mcp/
├── cmd/copilot-handler/
│   ├── main.go           # MCP server with Streamable HTTP
│   └── cbs_client.go     # CBS API client
├── Dockerfile.copilot    # Container build
├── copilot-studio-schema.yaml  # OpenAPI schema
└── README.md
```

See [DEPLOYMENT.md](DEPLOYMENT.md) for complete instructions.

## License

[Apache License 2.0](/LICENSE)

© 2025 David Stotijn
