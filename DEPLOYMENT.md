# Deployment Guide

Complete guide for deploying the MCP server to Azure Container Apps.

## Prerequisites

- [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) installed
- [Docker](https://docs.docker.com/get-docker/) installed (for local testing)
- Azure subscription with appropriate permissions

## Quick Start

### 1. Set Environment Variables

```bash
SUBSCRIPTION="Kodify - Partner Subscription (experiments & presales)"
RESOURCE_GROUP="Roodhals-PoC"
CONTAINER_APP_NAME="roodhals-mcp"
REGISTRY_NAME="roodhalsmcp"
LOCATION="westeurope"
CONTAINER_APP_ENV="roodhals-mcp-env"
```

### 2. Login and Set Subscription

```bash
az login
az account set --subscription "$SUBSCRIPTION"
```

### 3. Create Resources (First Time Only)

```bash
# Create resource group
az group create \
  --name "$RESOURCE_GROUP" \
  --location "$LOCATION"

# Create Azure Container Registry
az acr create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$REGISTRY_NAME" \
  --sku Basic \
  --location "$LOCATION" \
  --admin-enabled true

# Create Container Apps environment
az containerapp env create \
  --name "$CONTAINER_APP_ENV" \
  --resource-group "$RESOURCE_GROUP" \
  --location "$LOCATION"
```

## Deployment Options

### Option A: Deploy for Copilot Studio (Recommended)

Use this for Microsoft Copilot Studio integration with Streamable transport.

```bash
# Build and push image
az acr build \
  --registry "$REGISTRY_NAME" \
  --image mcp-server:copilot \
  --file Dockerfile.copilot .

# Create or update Container App
az containerapp create \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --environment "$CONTAINER_APP_ENV" \
  --image "$REGISTRY_NAME.azurecr.io/mcp-server:copilot" \
  --target-port 8080 \
  --ingress external \
  --registry-server "$REGISTRY_NAME.azurecr.io" \
  --query properties.configuration.ingress.fqdn \
  --output tsv
```

**If the app already exists, use update instead:**

```bash
az containerapp update \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --image "$REGISTRY_NAME.azurecr.io/mcp-server:copilot"
```

### Option B: Deploy Standard Version (SSE Transport)

Use this for MCP clients like Claude Desktop, VS Code extensions, etc.

```bash
# Build and push image
az acr build \
  --registry "$REGISTRY_NAME" \
  --image mcp-server:latest \
  --file Dockerfile .

# Create or update Container App
az containerapp create \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --environment "$CONTAINER_APP_ENV" \
  --image "$REGISTRY_NAME.azurecr.io/mcp-server:latest" \
  --target-port 8080 \
  --ingress external \
  --registry-server "$REGISTRY_NAME.azurecr.io" \
  --query properties.configuration.ingress.fqdn \
  --output tsv
```

## Get Deployment URL

```bash
az containerapp show \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --query properties.configuration.ingress.fqdn \
  --output tsv
```

Example output: `roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io`

## Update Deployment

When you make code changes, rebuild and redeploy:

```bash
# For Copilot Studio version
az acr build \
  --registry "$REGISTRY_NAME" \
  --image mcp-server:copilot \
  --file Dockerfile.copilot .

az containerapp update \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --image "$REGISTRY_NAME.azurecr.io/mcp-server:copilot"

# Or for standard version
az acr build \
  --registry "$REGISTRY_NAME" \
  --image mcp-server:latest \
  --file Dockerfile .

az containerapp update \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --image "$REGISTRY_NAME.azurecr.io/mcp-server:latest"
```

## Local Testing

### Test Copilot Studio Version

```bash
# Build locally
docker build -t mcp-server:copilot -f Dockerfile.copilot .

# Run locally
docker run -p 8080:8080 mcp-server:copilot

# Test endpoints
curl http://localhost:8080/health
curl http://localhost:8080/
```

### Test Standard Version

```bash
# Build locally
docker build -t mcp-server:latest .

# Run locally
docker run -p 8080:8080 mcp-server:latest
```

## Verify Deployment

### Check Health

```bash
FQDN=$(az containerapp show \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --query properties.configuration.ingress.fqdn \
  --output tsv)

curl https://$FQDN/health
```

### Test MCP Endpoint (Copilot Studio version)

```bash
curl -X POST https://$FQDN/mcp \
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

## View Logs

```bash
# Stream logs
az containerapp logs show \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --follow

# View recent logs
az containerapp logs show \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --tail 100
```

## Scaling Configuration

Container Apps auto-scales by default. To configure:

```bash
# Set min/max replicas
az containerapp update \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --min-replicas 0 \
  --max-replicas 10
```

## Environment Variables

Add environment variables to your deployment:

```bash
az containerapp update \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --set-env-vars "KEY1=value1" "KEY2=value2"
```

## Cost Management

### View Current Costs

```bash
az consumption usage list \
  --start-date $(date -u -d '30 days ago' '+%Y-%m-%d') \
  --end-date $(date -u '+%Y-%m-%d') \
  | jq '[.[] | select(.instanceName | contains("roodhals-mcp"))]'
```

### Estimated Monthly Costs

- **Container Apps (Consumption)**: ~$0 for low traffic (first 180,000 vCPU-seconds free)
- **Container Registry (Basic)**: ~$5/month
- **Log Analytics**: ~$2-5/month (depending on usage)

**Total**: ~$7-10/month for low-traffic usage

### Stop Container App (to save costs)

```bash
# Scale to 0 replicas
az containerapp update \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --min-replicas 0 \
  --max-replicas 0
```

## Cleanup

To delete all resources:

```bash
# Delete the entire resource group (removes everything)
az group delete \
  --name "$RESOURCE_GROUP" \
  --yes \
  --no-wait

# Or delete individual resources
az containerapp delete \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --yes

az acr delete \
  --name "$REGISTRY_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --yes
```

## Troubleshooting

### "Image not found" error

Make sure the image was built and pushed:
```bash
az acr repository list --name "$REGISTRY_NAME" --output table
az acr repository show-tags --name "$REGISTRY_NAME" --repository mcp-server --output table
```

### "Authentication failed" error

Check ACR credentials are configured:
```bash
az acr credential show --name "$REGISTRY_NAME"
```

### Container not starting

Check logs:
```bash
az containerapp logs show \
  --name "$CONTAINER_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --tail 50
```

### "Port already in use" (local testing)

Kill the process using port 8080:
```bash
lsof -ti:8080 | xargs kill -9
```

## Next Steps

- **For Copilot Studio**: See [COPILOT_STUDIO_SETUP.md](COPILOT_STUDIO_SETUP.md)
- **For Docker details**: See [DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md)
- **For Azure Functions**: See [AZURE_DEPLOYMENT.md](AZURE_DEPLOYMENT.md)

## Makefile Commands

Quick commands for common tasks:

```bash
# Build Docker image locally
make build-docker

# Run Docker container locally
make docker-run

# Build for Azure Functions
make build-azure

# Clean up
make clean
```

## Architecture

```
┌─────────────────────────────────────────────┐
│         Azure Container Apps                │
│  ┌───────────────────────────────────────┐  │
│  │  MCP Server Container                 │  │
│  │  - Auto-scaling (0-10 replicas)       │  │
│  │  - HTTPS ingress                      │  │
│  │  - Health monitoring                  │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
                    ▲
                    │
┌─────────────────────────────────────────────┐
│    Azure Container Registry (ACR)           │
│    - Private Docker registry                │
│    - Image: mcp-server:copilot              │
└─────────────────────────────────────────────┘
                    ▲
                    │
┌─────────────────────────────────────────────┐
│         Local Development                   │
│    - Build with Docker                      │
│    - Push to ACR                            │
│    - Deploy to Container Apps               │
└─────────────────────────────────────────────┘
```

## Security Best Practices

1. **Use managed identities** instead of admin credentials when possible
2. **Enable HTTPS only** (already configured)
3. **Restrict ingress** to specific IPs if needed:
   ```bash
   az containerapp ingress access-restriction set \
     --name "$CONTAINER_APP_NAME" \
     --resource-group "$RESOURCE_GROUP" \
     --rule-name "allow-office" \
     --ip-address "1.2.3.4/32" \
     --action Allow
   ```
4. **Use secrets** for sensitive data instead of environment variables
5. **Enable diagnostic logs** for monitoring

## Support

- Azure Container Apps: https://learn.microsoft.com/en-us/azure/container-apps/
- MCP Specification: https://modelcontextprotocol.io/
- Copilot Studio MCP: https://learn.microsoft.com/en-us/microsoft-copilot-studio/agent-extend-action-mcp
