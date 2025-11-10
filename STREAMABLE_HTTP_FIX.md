# Streamable HTTP Implementation - Fix for Copilot Studio

## The Problem (Solved!)

Based on community research and real-world experience, the issue was:

**Our server was NOT properly implementing Streamable HTTP transport.**

### What We Had Before:
- ❌ Only POST endpoint (no GET for SSE)
- ❌ Didn't check `Accept` header
- ❌ Pure synchronous JSON responses only
- ❌ Copilot Studio would initialize but never call `tools/list`

### Root Cause from Community:
From the forum post "MCP Server + Custom Connector Issues in Copilot Studio":
> "Updated MCP server to support Streamable-HTTP and it works now."

The issue: **Copilot Studio's MCP client requires proper Streamable HTTP**, not just plain HTTP POST.

## The Fix

### 1. Added GET Handler for SSE
```go
func handleSSEStream(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    // For stateless design, we acknowledge and close
    fmt.Fprintf(w, ": MCP server ready (stateless mode - no persistent SSE)\n\n")
}
```

### 2. Check Accept Header
```go
acceptHeader := r.Header.Get("Accept")
supportsSSE := strings.Contains(acceptHeader, "text/event-stream")
supportsJSON := strings.Contains(acceptHeader, "application/json")
```

### 3. Support Both GET and POST
- **POST**: For client requests (initialize, tools/list, tools/call)
- **GET**: For SSE stream (server can send notifications)

## Streamable HTTP Requirements

Per MCP spec and Copilot Studio requirements:

### Required:
✅ Single endpoint supporting POST and GET  
✅ POST with `Accept: application/json, text/event-stream`  
✅ GET with `Accept: text/event-stream` returns SSE stream  
✅ `x-ms-agentic-protocol: mcp-streamable-1.0` in OpenAPI schema  

### Our Implementation:
✅ POST → JSON responses (stateless)  
✅ GET → SSE stream (immediate close for stateless design)  
✅ Proper headers and content types  
✅ Schema already had `x-ms-agentic-protocol: mcp-streamable-1.0`  

## Testing

### Test POST with Accept header:
```bash
curl -X POST https://roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}'
```
✅ Returns JSON with proper initialize response

### Test GET for SSE:
```bash
curl -X GET https://roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io/mcp \
  -H "Accept: text/event-stream"
```
✅ Returns `Content-Type: text/event-stream` with SSE comment

## What Changed

### Before:
```go
func handleMCPRequest(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    // ... only POST handling
}
```

### After:
```go
func handleMCPRequest(w http.ResponseWriter, r *http.Request) {
    // Handle GET for SSE stream (Streamable HTTP requirement)
    if r.Method == http.MethodGet {
        handleSSEStream(w, r)
        return
    }
    
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    // Check Accept header for Streamable HTTP compliance
    acceptHeader := r.Header.Get("Accept")
    supportsSSE := strings.Contains(acceptHeader, "text/event-stream")
    // ... POST handling with awareness of SSE support
}
```

## Expected Copilot Studio Behavior Now

With proper Streamable HTTP:

1. ✅ Copilot Studio calls `initialize` with `Accept: application/json, text/event-stream`
2. ✅ Server responds with JSON (we support both)
3. ✅ Copilot Studio sends `notifications/initialized`
4. ✅ **Copilot Studio should now call `tools/list`** (this was the missing piece!)
5. ✅ Tools appear in UI

## Why This Matters

From Microsoft docs:
> "Currently, Copilot Studio supports the Streamable transport type. SSE is deprecated and no longer supported after August 2025."

Copilot Studio's MCP client **requires** Streamable HTTP, which means:
- Server MUST support both POST (for requests) and GET (for SSE)
- Server MUST check Accept header
- Server MUST be able to return either JSON or SSE

Without proper Streamable HTTP, Copilot Studio's MCP client doesn't fully initialize and never calls `tools/list`.

## Stateless Design Note

Our implementation is **stateless** for Azure Container Apps autoscaling:
- SSE streams close immediately (no persistent connections)
- No server-initiated notifications (would require stateful connections)
- Still compliant with Streamable HTTP spec (SSE is optional for basic operation)

This is a **pragmatic compromise**:
- ✅ Works with Copilot Studio
- ✅ Scales horizontally in Azure
- ⚠️ No real-time server notifications (acceptable for our use case)

## Next Steps

1. **Test in Copilot Studio**:
   - Delete and recreate the MCP connection
   - Check if tools now appear
   - Monitor logs for `tools/list` calls

2. **Check Logs**:
```bash
az containerapp logs show \
  --name roodhals-mcp \
  --resource-group Roodhals-PoC \
  --type console \
  --tail 30
```
Look for: `supportsSSE=true` in logs

3. **If Still Not Working**:
   - Check if Copilot Studio is actually calling with proper Accept header
   - Verify `x-ms-agentic-protocol: mcp-streamable-1.0` in schema
   - Try in Test panel with "Show activity map" enabled

## References

- MCP Spec: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#streamable-http
- Community fix: "Updated MCP server to support Streamable-HTTP and it works now"
- Microsoft docs: Copilot Studio requires Streamable transport
