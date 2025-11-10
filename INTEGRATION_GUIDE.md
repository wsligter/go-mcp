# Integrating CBS MCP Server with Copilot Studio

## Current Situation

You have two separate MCP servers:
1. **go-mcp** - Generic MCP server framework (this repo)
2. **mcp-cbs-cijfers-open-data** - CBS Open Data API MCP server

## Integration Options

### Option 1: Copy CBS Tools (Recommended for Copilot Studio)

Copy the CBS client and tool implementations directly into the Copilot handler.

**Steps:**

1. Copy CBS client code from `mcp-cbs-cijfers-open-data/main.go` into `cmd/copilot-handler/`
2. Extract the tool creation functions (createQueryDatasetsTools, etc.)
3. Register them in the Copilot handler

**Pros:**
- Single deployment
- Works with Copilot Studio's Streamable transport
- Simpler to maintain

**Cons:**
- Code duplication
- Need to manually sync updates

### Option 2: Run Both Servers Separately

Deploy both MCP servers and connect them separately to Copilot Studio.

**Steps:**

1. Deploy go-mcp server (already done): `https://roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io/mcp`
2. Deploy mcp-cbs-cijfers-open-data server separately
3. Add both as separate MCP connectors in Copilot Studio

**Pros:**
- Keep servers independent
- Easier to update each separately
- Clear separation of concerns

**Cons:**
- Two deployments to manage
- Two connectors in Copilot Studio
- More complex setup

### Option 3: Create a Combined Server

Merge both into a single MCP server with all tools.

**Steps:**

1. Create `cmd/combined-handler/main.go`
2. Import CBS client code
3. Register both weather (example) and CBS tools
4. Deploy as single server

**Pros:**
- Single endpoint for all tools
- One connector in Copilot Studio
- Clean architecture

**Cons:**
- More complex codebase
- Larger deployment

## Recommended Approach

For Copilot Studio integration, I recommend **Option 3: Combined Server**.

Here's why:
- Copilot Studio works best with a single MCP connector
- All CBS tools available in one place
- Simpler for end users

## Quick Start: Deploy CBS Server Separately

If you want to keep them separate for now:

```bash
# In mcp-cbs-cijfers-open-data directory
cd /Users/wouter/workdir/Roodhals/mcp-cbs-cijfers-open-data

# Create Dockerfile
cat > Dockerfile <<'EOF'
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o mcp-server .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/mcp-server .
EXPOSE 8080
CMD ["./mcp-server", "--stdio=false", "--sse"]
EOF

# Build and deploy to Azure
az acr build \
  --registry roodhalsmcp \
  --image mcp-cbs-server:latest \
  --file Dockerfile .

# Create new Container App for CBS
az containerapp create \
  --name roodhals-mcp-cbs \
  --resource-group Roodhals-PoC \
  --environment roodhals-mcp-env \
  --image roodhalsmcp.azurecr.io/mcp-cbs-server:latest \
  --target-port 8080 \
  --ingress external \
  --registry-server roodhalsmcp.azurecr.io
```

Then add both servers to Copilot Studio:
1. `roodhals-mcp` - Generic tools (weather example)
2. `roodhals-mcp-cbs` - CBS Open Data tools

## Next Steps

Choose your preferred option and I'll help you implement it!
