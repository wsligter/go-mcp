# GitHub Actions Setup for Azure Deployment

## Prerequisites

You've already created:
- ✅ Resource Group: `Roodhals-PoC`
- ✅ Container Registry: `roodhalsmcp`
- ✅ Docker image built and pushed

## Step 1: Update Workflow Variables

Edit `.github/workflows/azure-docker-deploy.yml` to match your settings:

```yaml
env:
  REGISTRY_NAME: 'roodhalsmcp'        # Your ACR name
  IMAGE_NAME: 'mcp-server'
  RESOURCE_GROUP: 'Roodhals-PoC'      # Your resource group
  CONTAINER_APP_NAME: 'roodhals-mcp'  # Your container app name
  CONTAINER_APP_ENV: 'roodhals-mcp-env'
```

## Step 2: Create Azure Service Principal

Run this command to create credentials for GitHub Actions:

```bash
az ad sp create-for-rbac \
  --name "github-actions-roodhals-mcp" \
  --role contributor \
  --scopes /subscriptions/2235f7c9-680d-4c2f-a568-76167fcff8e1/resourceGroups/Roodhals-PoC \
  --sdk-auth
```

This outputs JSON like:
```json
{
  "clientId": "xxx",
  "clientSecret": "xxx",
  "subscriptionId": "2235f7c9-680d-4c2f-a568-76167fcff8e1",
  "tenantId": "xxx",
  ...
}
```

**Copy the entire JSON output** - you'll need it in the next step.

## Step 3: Add Secret to GitHub

1. Go to your GitHub repository: `https://github.com/dstotijn/go-mcp`
2. Click **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. Name: `AZURE_CREDENTIALS`
5. Value: Paste the entire JSON from Step 2
6. Click **Add secret**

## Step 4: Grant ACR Access to Service Principal

The service principal needs permission to push images to ACR:

```bash
# Get the service principal's client ID from the JSON output above
SP_CLIENT_ID="<clientId from JSON>"

# Grant AcrPush role
az role assignment create \
  --assignee $SP_CLIENT_ID \
  --role AcrPush \
  --scope /subscriptions/2235f7c9-680d-4c2f-a568-76167fcff8e1/resourceGroups/Roodhals-PoC/providers/Microsoft.ContainerRegistry/registries/roodhalsmcp
```

## Step 5: Test the Workflow

### Option A: Push to Branch
```bash
git add .
git commit -m "Update workflow configuration"
git push origin azure-functions-deployment
```

### Option B: Manual Trigger
1. Go to **Actions** tab in GitHub
2. Select **Deploy Docker to Azure Container Apps**
3. Click **Run workflow**
4. Select branch: `azure-functions-deployment`
5. Click **Run workflow**

## What Happens Next

Every time you push to the `azure-functions-deployment` branch:

1. ✅ GitHub Actions automatically triggers
2. ✅ Builds Docker image in Azure (no local Docker needed)
3. ✅ Pushes to ACR with commit SHA tag
4. ✅ Deploys to Container Apps
5. ✅ Shows deployment URL in logs

## Monitoring

View workflow runs:
- GitHub: **Actions** tab
- Azure: Portal → Container Apps → Revisions

## Troubleshooting

### "AZURE_CREDENTIALS secret not found"
- Make sure you added the secret in Step 3

### "Authorization failed"
- Verify service principal has Contributor role
- Check ACR permissions (Step 4)

### "Container app not found"
- First deployment: Create the Container App environment manually:
  ```bash
  az containerapp env create \
    --name roodhals-mcp-env \
    --resource-group Roodhals-PoC \
    --location westeurope
  ```

## Benefits

- 🚀 **Zero-touch deployment** - Push code, get deployed app
- 🔄 **Automatic rollback** - Keep previous revisions
- 📊 **Deployment history** - Track all deployments
- 🔒 **Secure** - No credentials in code
- ⚡ **Fast** - Builds in Azure (parallel, cached layers)

## Cost

GitHub Actions is free for public repos, 2000 minutes/month for private repos.
