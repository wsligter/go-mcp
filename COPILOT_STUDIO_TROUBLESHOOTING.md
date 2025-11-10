# Copilot Studio MCP Integration Troubleshooting

## Current Status

✅ **MCP Server is working correctly**
- Server URL: `https://roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io/mcp`
- Protocol: MCP 2024-11-05 (Streamable transport)
- Tools: 4 CBS tools available
- Logs show Copilot Studio IS connecting and calling initialize

❌ **Copilot Studio UI shows "No tools available"**
- Server is being called successfully
- Initialize response includes 4 tools
- But Copilot Studio UI doesn't display them

## Verified Working

```bash
# Test initialize - returns 4 tools
curl -X POST https://roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"copilot","version":"1.0"}},"id":1}' \
  | jq '.result.tools | length'
# Returns: 4

# Test tools/list
curl -X POST https://roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":1}' \
  | jq '.result.tools[] | .name'
# Returns: "get_catalogs", "query_datasets", "get_dimensions", "query_observations"
```

## Issue Analysis

From the logs, we can see:
1. Copilot Studio calls `initialize` ✅
2. Server returns 4 tools in response ✅  
3. Copilot Studio sends `notifications/initialized` ✅
4. **Copilot Studio NEVER calls `tools/list`** ❌

This suggests Copilot Studio's MCP onboarding wizard may have a bug or expects tools in a different format.

## Solution Options

### Option 1: Try Custom Connector (Recommended)

Instead of using the MCP onboarding wizard, create a custom connector manually:

1. **Go to Power Apps**: https://make.powerapps.com
2. **Navigate to**: Data → Custom connectors → New custom connector → Import an OpenAPI file
3. **Upload**: `copilot-studio-schema.yaml` from this repo
4. **Configure**:
   - Host: `roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io`
   - Base URL: `/`
   - Authentication: None
5. **Test the connector** in Power Apps
6. **Add to Copilot Studio**: Go back to Copilot Studio → Tools → Add tool → Select your custom connector

### Option 2: Check Generative Orchestration

The docs state: "Generative Orchestration must be enabled to use MCP"

1. In Copilot Studio, go to **Settings** → **Generative AI**
2. Ensure **Generative Orchestration** is **enabled**
3. Save and try refreshing the MCP connection

### Option 3: Try Different Browser/Clear Cache

Sometimes Copilot Studio UI caches incorrectly:

1. **Clear browser cache** completely
2. **Try in incognito/private mode**
3. **Try a different browser** (Edge, Chrome)
4. **Hard refresh** (Ctrl+Shift+R or Cmd+Shift+R)

### Option 4: Contact Microsoft Support

If none of the above work, this may be a bug in Copilot Studio's MCP implementation. The server is working correctly per the MCP spec.

**Evidence to provide**:
- Server logs show successful initialize calls with 4 tools advertised
- Direct curl tests show all 4 tools are available
- Server implements MCP 2024-11-05 spec correctly
- Copilot Studio never calls `tools/list` after initialization

## Available Tools

When working, these 4 tools should appear:

1. **get_catalogs**
   - Description: Retrieves all available CBS data catalogs
   - Parameters: None

2. **query_datasets**
   - Description: Lists available datasets from CBS Open Data API
   - Parameters: catalog (required), filter, search, top, skip

3. **get_dimensions**
   - Description: Retrieves dimensions for a specific CBS dataset
   - Parameters: catalog (required), dataset (required)

4. **query_observations**
   - Description: Queries statistical observations from a CBS dataset
   - Parameters: catalog (required), dataset (required), filter, select, top

## Server Logs

To view what Copilot Studio is actually calling:

```bash
az containerapp logs show \
  --name roodhals-mcp \
  --resource-group Roodhals-PoC \
  --type console \
  --tail 30
```

Look for lines like:
- `MCP Request: method=initialize` - Copilot Studio connecting
- `Advertising 4 tools in initialize response` - Server sending tools
- `MCP Request: method=tools/list` - Should appear but doesn't

## Next Steps

1. Try Option 1 (Custom Connector) first
2. If that doesn't work, verify Generative Orchestration is enabled
3. If still not working, this is likely a Copilot Studio bug - contact Microsoft support with the evidence above
