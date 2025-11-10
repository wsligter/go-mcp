# Docker Deployment Guide for Azure

This guide shows how to deploy the go-mcp server to Azure using Docker containers - a simpler alternative to custom handlers.

## Why Docker?

**Advantages over Custom Handlers:**
- ✅ Simpler configuration (no `host.json` or `function.json` needed)
- ✅ Consistent builds across environments
- ✅ Easier local testing
- ✅ Better for Azure Container Apps or App Service
- ✅ More portable (works on any cloud provider)

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed
- [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) installed
- An Azure subscription

## Build Docker Image

Build the image locally:

```bash
docker build -t mcp-server:latest .
```

## Local Testing

Run the container locally:

```bash
docker run -p 8080:8080 mcp-server:latest
```

The MCP server will be available at `http://localhost:8080`

## Deployment Options

### Option 1: Azure Container Apps (Recommended)

Azure Container Apps is serverless and simpler than Kubernetes.

1. **Login to Azure:**
   ```bash
   az login
   ```

2. **Create a resource group:**
   ```bash
   az group create --name mcp-rg --location eastus
   ```

3. **Create a Container Apps environment:**
   ```bash
   az containerapp env create \
     --name mcp-env \
     --resource-group mcp-rg \
     --location eastus
   ```

4. **Create Azure Container Registry:**
   ```bash
   az acr create \
     --resource-group mcp-rg \
     --name mcpregistry<unique-id> \
     --sku Basic \
     --admin-enabled true
   ```

5. **Build and push image to ACR:**
   ```bash
   az acr build \
     --registry mcpregistry<unique-id> \
     --image mcp-server:latest \
     --file Dockerfile .
   ```

6. **Deploy to Container Apps:**
   ```bash
   az containerapp create \
     --name mcp-server \
     --resource-group mcp-rg \
     --environment mcp-env \
     --image mcpregistry<unique-id>.azurecr.io/mcp-server:latest \
     --target-port 8080 \
     --ingress external \
     --registry-server mcpregistry<unique-id>.azurecr.io \
     --query properties.configuration.ingress.fqdn
   ```

Your server will be available at the returned FQDN.

### Option 2: Azure App Service (Web App for Containers)

1. **Create App Service Plan:**
   ```bash
   az appservice plan create \
     --name mcp-plan \
     --resource-group mcp-rg \
     --is-linux \
     --sku B1
   ```

2. **Create ACR and push image** (same as steps 4-5 above)

3. **Create Web App:**
   ```bash
   az webapp create \
     --resource-group mcp-rg \
     --plan mcp-plan \
     --name mcp-webapp-<unique-id> \
     --deployment-container-image-name mcpregistry<unique-id>.azurecr.io/mcp-server:latest
   ```

4. **Configure ACR credentials:**
   ```bash
   az webapp config container set \
     --name mcp-webapp-<unique-id> \
     --resource-group mcp-rg \
     --docker-custom-image-name mcpregistry<unique-id>.azurecr.io/mcp-server:latest \
     --docker-registry-server-url https://mcpregistry<unique-id>.azurecr.io \
     --docker-registry-server-user mcpregistry<unique-id> \
     --docker-registry-server-password $(az acr credential show --name mcpregistry<unique-id> --query passwords[0].value -o tsv)
   ```

Your server will be available at `https://mcp-webapp-<unique-id>.azurewebsites.net`

### Option 3: Azure Kubernetes Service (AKS)

For production workloads requiring orchestration:

1. **Create AKS cluster:**
   ```bash
   az aks create \
     --resource-group mcp-rg \
     --name mcp-cluster \
     --node-count 1 \
     --enable-addons monitoring \
     --generate-ssh-keys \
     --attach-acr mcpregistry<unique-id>
   ```

2. **Get credentials:**
   ```bash
   az aks get-credentials --resource-group mcp-rg --name mcp-cluster
   ```

3. **Deploy using kubectl:**
   ```bash
   kubectl create deployment mcp-server --image=mcpregistry<unique-id>.azurecr.io/mcp-server:latest
   kubectl expose deployment mcp-server --type=LoadBalancer --port=80 --target-port=8080
   ```

## GitHub Actions CI/CD

Update `.github/workflows/azure-deploy.yml` to use Docker:

```yaml
name: Deploy Docker to Azure

on:
  push:
    branches:
      - azure-functions-deployment
  workflow_dispatch:

env:
  REGISTRY_NAME: mcpregistry
  IMAGE_NAME: mcp-server
  RESOURCE_GROUP: mcp-rg
  CONTAINER_APP_NAME: mcp-server

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    
    - name: Login to Azure
      uses: azure/login@v1
      with:
        creds: ${{ secrets.AZURE_CREDENTIALS }}
    
    - name: Build and push to ACR
      run: |
        az acr build \
          --registry ${{ env.REGISTRY_NAME }} \
          --image ${{ env.IMAGE_NAME }}:${{ github.sha }} \
          --image ${{ env.IMAGE_NAME }}:latest \
          --file Dockerfile .
    
    - name: Deploy to Container Apps
      run: |
        az containerapp update \
          --name ${{ env.CONTAINER_APP_NAME }} \
          --resource-group ${{ env.RESOURCE_GROUP }} \
          --image ${{ env.REGISTRY_NAME }}.azurecr.io/${{ env.IMAGE_NAME }}:${{ github.sha }}
```

## Configuration

Set environment variables in Azure:

```bash
# For Container Apps
az containerapp update \
  --name mcp-server \
  --resource-group mcp-rg \
  --set-env-vars "KEY=value"

# For App Service
az webapp config appsettings set \
  --name mcp-webapp-<unique-id> \
  --resource-group mcp-rg \
  --settings KEY=value
```

## Cost Comparison

- **Container Apps**: ~$0 for low traffic (consumption-based)
- **App Service B1**: ~$13/month (always running)
- **AKS**: ~$73/month minimum (includes VM costs)

## Monitoring

View logs:

```bash
# Container Apps
az containerapp logs show \
  --name mcp-server \
  --resource-group mcp-rg \
  --follow

# App Service
az webapp log tail \
  --name mcp-webapp-<unique-id> \
  --resource-group mcp-rg
```

## Notes

- The `examples/` folder is excluded via `.dockerignore`
- Multi-stage build keeps the final image small (~15MB)
- Container Apps is recommended for serverless, cost-effective deployment
- App Service is good for simple, always-on workloads
- AKS is best for complex, production-grade deployments
