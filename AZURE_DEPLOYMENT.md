# Azure Functions Deployment Guide

This guide explains how to deploy the go-mcp server to Azure Functions.

## Prerequisites

- [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) installed
- [Azure Functions Core Tools](https://docs.microsoft.com/en-us/azure/azure-functions/functions-run-local) installed
- An Azure subscription

## Project Structure

The following files are required for Azure Functions deployment:

- `host.json` - Azure Functions host configuration
- `MCPFunction/function.json` - HTTP trigger function configuration
- `cmd/azure-handler/main.go` - Custom handler entry point
- `.funcignore` - Files to exclude from deployment (includes `examples/`)

## Build

Build the custom handler for Linux (Azure Functions runtime):

```bash
make build-azure
```

This creates a `handler` binary that Azure Functions will execute.

## Local Testing

Test the function locally:

```bash
func start
```

The MCP server will be available at `http://localhost:7071/api/MCPFunction`

## Deploy to Azure

1. **Login to Azure:**
   ```bash
   az login
   ```

2. **Create a resource group (if needed):**
   ```bash
   az group create --name mcp-rg --location eastus
   ```

3. **Create a storage account:**
   ```bash
   az storage account create \
     --name mcpstorage<unique-id> \
     --resource-group mcp-rg \
     --location eastus \
     --sku Standard_LRS
   ```

4. **Create a Function App:**
   ```bash
   az functionapp create \
     --resource-group mcp-rg \
     --consumption-plan-location eastus \
     --runtime custom \
     --functions-version 4 \
     --name mcp-function-app-<unique-id> \
     --storage-account mcpstorage<unique-id> \
     --os-type Linux
   ```

5. **Deploy the function:**
   ```bash
   func azure functionapp publish mcp-function-app-<unique-id>
   ```

## Configuration

After deployment, you can configure application settings:

```bash
az functionapp config appsettings set \
  --name mcp-function-app-<unique-id> \
  --resource-group mcp-rg \
  --settings "SETTING_NAME=value"
```

## Accessing the Deployed Function

Your MCP server will be available at:
```
https://mcp-function-app-<unique-id>.azurewebsites.net/api/MCPFunction
```

## Notes

- The `examples/` folder is excluded from deployment via `.funcignore`
- Only the core library files and the custom handler are deployed
- The handler uses SSE (Server-Sent Events) transport for Azure Functions compatibility
- Customize `cmd/azure-handler/main.go` to add your own tools, resources, and prompts

## Troubleshooting

View logs:
```bash
func azure functionapp logstream mcp-function-app-<unique-id>
```

Or use Azure Portal to view Application Insights logs.
