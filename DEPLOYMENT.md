# Deployment Guide

## Azure Container Apps Deployment

### Prerequisites

- Azure CLI installed and logged in
- Azure Container Registry
- Azure Container Apps environment

### Build and Deploy

```bash
# 1. Build and push to Azure Container Registry
az acr build --registry roodhalsmcp \
  --image mcp-server:copilot \
  --file Dockerfile.copilot .

# 2. Update Container App
az containerapp update \
  --name roodhals-mcp \
  --resource-group Roodhals-PoC
```

### Verify Deployment

```bash
# Check logs
az containerapp logs show \
  --name roodhals-mcp \
  --resource-group Roodhals-PoC \
  --type console \
  --tail 20

# Test endpoint
curl https://roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io/health
```

## Copilot Studio Integration

### Add MCP Server

1. Open Copilot Studio
2. Go to **Tools** → **Add a tool** → **Model Context Protocol**
3. Enter server URL: `https://roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io/mcp`
4. Select **Streamable** transport
5. Authentication: **None**
6. Save

### Test

Ask Copilot: "What was the population of the Netherlands in 2022?"

Expected behavior:
1. Searches CBS datasets with `search="bevolking"`
2. Gets dimensions from found dataset
3. Queries observations with proper filters

## Local Development

### Run Locally

```bash
# Build
go build -o mcp-server ./cmd/copilot-handler/

# Run
./mcp-server
# Starts on http://localhost:8080
```

### Test Locally

```bash
# Initialize
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}'

# List tools
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":2}'

# Call tool
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_catalogs","arguments":{}},"id":3}'
```

## Environment Variables

None required - server uses CBS public API.

## Troubleshooting

### Tools not appearing in Copilot Studio

1. Refresh the MCP connection
2. Check server logs for errors
3. Verify URL is correct
4. Ensure Generative Orchestration is enabled in Copilot Studio settings

### Server errors

```bash
# Check logs
az containerapp logs show \
  --name roodhals-mcp \
  --resource-group Roodhals-PoC \
  --type console \
  --follow
```

## Architecture

- **Transport**: Streamable HTTP (MCP 2024-11-05)
- **Design**: Stateless for horizontal scaling
- **CBS API**: Direct HTTP calls to https://opendata.cbs.nl/ODataApi/odata
- **Port**: 8080
- **Health**: `/health` endpoint
