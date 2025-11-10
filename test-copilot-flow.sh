#!/bin/bash

# Test MCP server flow as Copilot Studio would do it
URL="https://roodhals-mcp.ashysea-5e0ea900.westeurope.azurecontainerapps.io/mcp"

echo "=== Step 1: Initialize ==="
curl -X POST "$URL" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"copilot-studio","version":"1.0"}},"id":1}' \
  | jq

echo -e "\n=== Step 2: Initialized Notification (no response expected) ==="
curl -X POST "$URL" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}' \
  -w "\nHTTP Status: %{http_code}\n"

echo -e "\n=== Step 3: List Tools ==="
curl -X POST "$URL" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":2}' \
  | jq '.result.tools[] | {name, description}'

echo -e "\n=== Step 4: List Resources ==="
curl -X POST "$URL" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"resources/list","params":{},"id":3}' \
  | jq

echo -e "\n=== Step 5: Call a tool (get_catalogs) ==="
curl -X POST "$URL" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_catalogs","arguments":{}},"id":4}' \
  | jq
