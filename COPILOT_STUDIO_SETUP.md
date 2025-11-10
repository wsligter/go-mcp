# Microsoft Copilot Studio Integration Guide

This guide explains how to connect your MCP server to Microsoft Copilot Studio.

## Important: Transport Type

**Copilot Studio requires "Streamable" transport, NOT SSE (Server-Sent Events).**

The original `cmd/azure-handler/main.go` uses SSE which is deprecated and not supported by Copilot Studio. Use `cmd/copilot-handler/main.go` instead.

## Prerequisites

- ✅ Azure Container Apps deployment (already done)
- ✅ MCP server running
- ✅ Microsoft Copilot Studio access
- ✅ Power Apps access (for custom connectors)

## Step 1: Deploy Copilot Studio Compatible Version

### Update your deployment to use the Copilot-compatible handler:

```bash
# Variables (already set)
REGISTRY_NAME="roodhalsmcp"
RESOURCE_GROUP="Roodhals-PoC"
CONTAINER_APP_NAME="roodhals-mcp"

# Build and push new image with Copilot handler
az acr build \
  --registry $REGISTRY_NAME \
  --image mcp-server:copilot \
  --file Dockerfile.copilot .

# Update Container App with new image
az containerapp update \
  --name $CONTAINER_APP_NAME \
  --resource-group $RESOURCE_GROUP \
  --image $REGISTRY_NAME.azurecr.io/mcp-server:copilot
```

### Get your deployment URL:

```bash
az containerapp show \
  --name $CONTAINER_APP_NAME \
  --resource-group $RESOURCE_GROUP \
  --query properties.configuration.ingress.fqdn \
  --output tsv
```

Expected output: `roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io`

## Step 2: Update OpenAPI Schema

Edit `copilot-studio-schema.yaml` and replace the host with your actual URL:

```yaml
host: <your-container-app-url>  # e.g., roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io
```

## Step 3: Connect to Copilot Studio

### Option A: Use MCP Onboarding Wizard (Recommended)

1. **Open Copilot Studio**
   - Go to https://copilotstudio.microsoft.com
   - Select your agent or create a new one

2. **Add MCP Server**
   - Go to **Tools** → **Add a tool** → **Model Context Protocol**
   - Click **Add MCP server**

3. **Configure Server Details**
   - **Name**: Roodhals MCP Server
   - **Description**: Weather tools and company data resources
   - **Server URL**: `https://<your-url>/mcp`
   - **Transport Type**: Streamable
   - **Protocol Version**: 2024-11-05

4. **Configure Authentication**
   - **Type**: No authentication (or configure if needed)
   - Click **Next**

5. **Test Connection**
   - Click **Test** to verify the connection
   - You should see available tools and resources

6. **Complete Setup**
   - Click **Create** to finish

### Option B: Create Custom Connector in Power Apps

1. **Go to Power Apps**
   - Navigate to https://make.powerapps.com
   - Select **Custom connectors** from the left menu

2. **Import OpenAPI File**
   - Click **New custom connector** → **Import an OpenAPI file**
   - Upload `copilot-studio-schema.yaml`
   - Click **Continue**

3. **Configure Connector**
   - **General tab**: Review name and description
   - **Security tab**: Set to "No authentication" (or configure as needed)
   - **Definition tab**: Verify the `/mcp` endpoint
   - **Test tab**: Create a connection and test

4. **Save and Use**
   - Click **Create connector**
   - Go back to Copilot Studio
   - Select **Tools** → **Add a tool** → **Custom connector**
   - Select your newly created connector

## Step 4: Add Tools and Resources to Your Agent

1. **In Copilot Studio**, go to your agent
2. Navigate to **Tools**
3. You should see the MCP server tools:
   - **get_weather** - Get weather information for a location

4. **Add to Agent**
   - Select the tools you want to use
   - Click **Add to agent**

5. **Test in Chat**
   - Go to **Test your agent**
   - Try: "What's the weather in Amsterdam?"
   - The agent should use the `get_weather` tool

## Available Tools

### get_weather
- **Description**: Get current weather information for a location
- **Input**: 
  - `location` (string, required): The location to get weather for
- **Output**: Weather information text

## Available Resources

### Company Data
- **URI**: `roodhals://data/company`
- **Description**: Company information and data from Roodhals systems
- **Type**: Text resource

## Troubleshooting

### "Connection failed" or "Transport not supported"

**Check the endpoint:**
```bash
curl -X POST https://<your-url>/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}'
```

You should get a JSON response with server capabilities.

### "SSE not supported" error

Make sure you're using `Dockerfile.copilot` which builds `cmd/copilot-handler/main.go`, not the SSE-based handler.

### Tools not appearing

1. Verify the MCP endpoint is accessible
2. Check that the server is returning tools in the `tools/list` response
3. Refresh the connection in Copilot Studio

### Authentication errors

If you need authentication:
1. Add API key or OAuth to the Container App
2. Update the OpenAPI schema with security definitions
3. Configure authentication in Power Apps connector

## Verify Deployment

Test the endpoints:

```bash
# Health check
curl https://<your-url>/health

# Server info
curl https://<your-url>/

# MCP initialize (test MCP protocol)
curl -X POST https://<your-url>/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "test", "version": "1.0"}
    },
    "id": 1
  }'
```

## Next Steps

1. **Customize Tools**: Edit `cmd/copilot-handler/main.go` to add your own tools
2. **Add Resources**: Implement resource handlers for your data sources
3. **Add Authentication**: Secure your MCP endpoint if needed
4. **Monitor Usage**: Use Azure Application Insights to track usage
5. **Scale**: Container Apps will auto-scale based on demand

## Key Differences from SSE Version

| Feature | SSE Version | Copilot Studio Version |
|---------|-------------|------------------------|
| Transport | Server-Sent Events | HTTP POST (Streamable) |
| Endpoint | `/` with SSE headers | `/mcp` POST endpoint |
| Compatibility | MCP clients (Claude, etc.) | Microsoft Copilot Studio |
| Connection | Long-lived | Request-response |
| Deployment | `Dockerfile` | `Dockerfile.copilot` |

## Resources

- [Copilot Studio MCP Documentation](https://learn.microsoft.com/en-us/microsoft-copilot-studio/agent-extend-action-mcp)
- [MCP Specification](https://modelcontextprotocol.io/introduction)
- [Power Apps Custom Connectors](https://learn.microsoft.com/en-us/connectors/custom-connectors/)
