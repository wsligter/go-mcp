// Copyright 2025 David Stotijn
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dstotijn/go-mcp"
)

const (
	serverName    = "roodhals-mcp-server"
	serverVersion = "0.1.0"
)

// JSON-RPC types
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var (
	mcpServer *mcp.Server
	tools     map[string]mcp.Tool
)

func main() {
	// Get port from environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx := context.Background()

	// Create MCP server
	mcpServer = mcp.NewServer(mcp.ServerConfig{
		Name:                    serverName,
		Version:                 serverVersion,
		ListResourcesFn:         handleListResourcesRequest,
		ReadResourceFn:          handleReadResourceRequest,
		ListResourceTemplatesFn: handleListResourceTemplatesRequest,
		ListPromptsFn:           handleListPromptsRequest,
		GetPromptFn:             handleGetPromptRequest,
		CompleteFn:              handleCompleteRequest,
	})

	mcpServer.Start(ctx)

	// Register CBS API tools - no MCP registration needed for Copilot Studio
	// Tools are exposed via JSON-RPC handlers below

	// Create HTTP handler for Copilot Studio (Streamable transport)
	http.HandleFunc("/mcp", handleMCPRequest)

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"server":  serverName,
			"version": serverVersion,
		})
	})

	// Root endpoint with info
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        serverName,
			"version":     serverVersion,
			"description": "MCP Server for Microsoft Copilot Studio",
			"endpoints": map[string]string{
				"mcp":    "/mcp (POST)",
				"health": "/health (GET)",
			},
		})
	})

	log.Printf("MCP server started (%q, %q) on port %s", serverName, serverVersion, port)
	log.Printf("Copilot Studio endpoint: http://localhost:%s/mcp", port)
	log.Printf("Health check: http://localhost:%s/health", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONRPCError(w, nil, -32700, "Parse error")
		return
	}
	defer r.Body.Close()

	// Parse JSON-RPC request
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Parse error: %v, body: %s", err, string(body))
		sendJSONRPCError(w, nil, -32700, "Parse error")
		return
	}

	log.Printf("MCP Request: method=%s, id=%v", req.Method, req.ID)

	// Handle the request based on method
	ctx := r.Context()
	var result interface{}

	switch req.Method {
	case "initialize":
		result = handleInitialize(ctx, req.Params)
	case "notifications/initialized":
		// Client notification after initialize - no response needed
		log.Printf("Client initialized notification received")
		return
	case "tools/list":
		result = handleToolsList(ctx, req.Params)
	case "tools/call":
		result = handleToolsCall(ctx, req.Params)
	case "resources/list":
		result = handleResourcesList(ctx, req.Params)
	case "resources/read":
		result = handleResourcesRead(ctx, req.Params)
	case "prompts/list":
		result = handlePromptsList(ctx, req.Params)
	case "ping":
		// Health check
		result = map[string]interface{}{"status": "ok"}
	default:
		log.Printf("Unknown method: %s", req.Method)
		sendJSONRPCError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
		return
	}

	// Send response
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func sendJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors still return 200
	json.NewEncoder(w).Encode(response)
}

func handleInitialize(ctx context.Context, params json.RawMessage) interface{} {
	log.Printf("Initialize called - returning capabilities and server info")
	
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": true,
			},
			"resources": map[string]interface{}{
				"subscribe":   false,
				"listChanged": true,
			},
			"prompts": map[string]interface{}{
				"listChanged": true,
			},
		},
		"serverInfo": map[string]interface{}{
			"name":    serverName,
			"version": serverVersion,
		},
		"instructions": "This server provides access to CBS (Statistics Netherlands) Open Data API. Use get_catalogs to list available catalogs, query_datasets to search datasets, get_dimensions to explore dataset structure, and query_observations to retrieve statistical data.",
	}
}

func handleToolsList(ctx context.Context, params json.RawMessage) interface{} {
	return map[string]interface{}{
		"tools": []map[string]interface{}{
			{
				"name":        "get_catalogs",
				"description": "Retrieves all available CBS data catalogs",
				"inputSchema": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			{
				"name":        "query_datasets",
				"description": "Lists available datasets from CBS Open Data API with filtering, sorting, and pagination",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"catalog": map[string]interface{}{
							"type":        "string",
							"description": "Catalog identifier (use 'CBS')",
						},
						"filter": map[string]interface{}{
							"type":        "string",
							"description": "OData $filter parameter (e.g., \"Status ne 'Gediscontinueerd'\")",
						},
						"search": map[string]interface{}{
							"type":        "string",
							"description": "Free-text search term",
						},
						"top": map[string]interface{}{
							"type":        "integer",
							"description": "Limit number of results (default: 10)",
						},
						"skip": map[string]interface{}{
							"type":        "integer",
							"description": "Skip N results for pagination",
						},
					},
					"required": []string{"catalog"},
				},
			},
			{
				"name":        "get_dimensions",
				"description": "Retrieves all dimensions (categories) for a specific CBS dataset",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"catalog": map[string]interface{}{
							"type":        "string",
							"description": "Catalog identifier",
						},
						"dataset": map[string]interface{}{
							"type":        "string",
							"description": "Dataset identifier (e.g., '83765NED')",
						},
					},
					"required": []string{"catalog", "dataset"},
				},
			},
			{
				"name":        "query_observations",
				"description": "Queries statistical observations from a CBS dataset with filtering and sorting",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"catalog": map[string]interface{}{
							"type":        "string",
							"description": "Catalog identifier",
						},
						"dataset": map[string]interface{}{
							"type":        "string",
							"description": "Dataset identifier",
						},
						"filter": map[string]interface{}{
							"type":        "string",
							"description": "OData $filter parameter",
						},
						"select": map[string]interface{}{
							"type":        "string",
							"description": "OData $select parameter (comma-separated fields)",
						},
						"top": map[string]interface{}{
							"type":        "integer",
							"description": "Limit number of results",
						},
					},
					"required": []string{"catalog", "dataset"},
				},
			},
		},
	}
}

func handleToolsCall(ctx context.Context, params json.RawMessage) interface{} {
	var callParams struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(params, &callParams); err != nil {
		return map[string]interface{}{
			"isError": true,
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "Invalid parameters",
				},
			},
		}
	}

	client := NewCBSClient()

	switch callParams.Name {
	case "get_catalogs":
		catalogs, err := client.GetCatalogs()
		if err != nil {
			return errorResponse(fmt.Sprintf("Failed to get catalogs: %v", err))
		}
		
		var text strings.Builder
		text.WriteString(fmt.Sprintf("Found %d CBS catalogs:\n\n", len(catalogs)))
		for i, cat := range catalogs {
			text.WriteString(fmt.Sprintf("%d. **%s** (ID: `%s`)\n", i+1, cat.Title, cat.Identifier))
			if cat.Description != "" {
				text.WriteString(fmt.Sprintf("   - %s\n", cat.Description))
			}
			text.WriteString("\n")
		}
		
		return successResponse(text.String())

	case "query_datasets":
		catalog, _ := callParams.Arguments["catalog"].(string)
		if catalog == "" {
			return errorResponse("catalog parameter is required")
		}
		
		queryOpts := make(map[string]string)
		if filter, ok := callParams.Arguments["filter"].(string); ok && filter != "" {
			queryOpts["$filter"] = filter
		}
		if search, ok := callParams.Arguments["search"].(string); ok && search != "" {
			queryOpts["$search"] = search
		}
		if top, ok := callParams.Arguments["top"].(float64); ok && top > 0 {
			queryOpts["$top"] = fmt.Sprintf("%.0f", top)
		} else {
			queryOpts["$top"] = "10" // Default
		}
		if skip, ok := callParams.Arguments["skip"].(float64); ok && skip > 0 {
			queryOpts["$skip"] = fmt.Sprintf("%.0f", skip)
		}
		queryOpts["$count"] = "true"
		
		datasets, totalCount, err := client.GetDatasetsWithQuery(catalog, queryOpts)
		if err != nil {
			return errorResponse(fmt.Sprintf("Failed to query datasets: %v", err))
		}
		
		skip := 0
		if s, ok := callParams.Arguments["skip"].(float64); ok {
			skip = int(s)
		}
		
		return successResponse(FormatDatasets(datasets, totalCount, skip))

	case "get_dimensions":
		catalog, _ := callParams.Arguments["catalog"].(string)
		dataset, _ := callParams.Arguments["dataset"].(string)
		
		if catalog == "" || dataset == "" {
			return errorResponse("catalog and dataset parameters are required")
		}
		
		dimensions, err := client.GetDimensions(catalog, dataset)
		if err != nil {
			return errorResponse(fmt.Sprintf("Failed to get dimensions: %v", err))
		}
		
		return successResponse(FormatDimensions(dimensions))

	case "query_observations":
		catalog, _ := callParams.Arguments["catalog"].(string)
		dataset, _ := callParams.Arguments["dataset"].(string)
		
		if catalog == "" || dataset == "" {
			return errorResponse("catalog and dataset parameters are required")
		}
		
		queryOpts := make(map[string]string)
		if filter, ok := callParams.Arguments["filter"].(string); ok && filter != "" {
			queryOpts["$filter"] = filter
		}
		if selectFields, ok := callParams.Arguments["select"].(string); ok && selectFields != "" {
			queryOpts["$select"] = selectFields
		}
		if top, ok := callParams.Arguments["top"].(float64); ok && top > 0 {
			queryOpts["$top"] = fmt.Sprintf("%.0f", top)
		} else {
			queryOpts["$top"] = "100" // Default
		}
		
		path := fmt.Sprintf("/%s/%s/Observations", catalog, dataset)
		result, err := client.ExecuteQuery(path, queryOpts)
		if err != nil {
			return errorResponse(fmt.Sprintf("Failed to query observations: %v", err))
		}
		
		limit := 100
		if top, ok := callParams.Arguments["top"].(float64); ok && top > 0 {
			limit = int(top)
		}
		
		formatted, err := FormatObservations(result, limit)
		if err != nil {
			return errorResponse(fmt.Sprintf("Failed to format observations: %v", err))
		}
		
		return successResponse(formatted)
	}

	return errorResponse("Tool not found")
}

func successResponse(text string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": text,
			},
		},
	}
}

func errorResponse(message string) map[string]interface{} {
	return map[string]interface{}{
		"isError": true,
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": message,
			},
		},
	}
}

func handleResourcesList(ctx context.Context, params json.RawMessage) interface{} {
	result, _ := handleListResourcesRequest(ctx, mcp.ListResourcesParams{})
	return result
}

func handleResourcesRead(ctx context.Context, params json.RawMessage) interface{} {
	var readParams struct {
		URI string `json:"uri"`
	}
	json.Unmarshal(params, &readParams)

	result, _ := handleReadResourceRequest(ctx, mcp.ReadResourceParams{URI: readParams.URI})
	return result
}

func handlePromptsList(ctx context.Context, params json.RawMessage) interface{} {
	result, _ := handleListPromptsRequest(ctx, mcp.ListPromptsParams{})
	return result
}

func handleListResourcesRequest(ctx context.Context, req mcp.ListResourcesParams) (*mcp.ListResourcesResult, error) {
	return &mcp.ListResourcesResult{
		Resources: []mcp.Resource{
			{
				Name:        "Company Data",
				URI:         "roodhals://data/company",
				MimeType:    "text/plain",
				Description: "Company information and data from Roodhals systems.",
				Annotations: &mcp.Annotations{
					Audience: []mcp.Role{mcp.RoleAssistant},
					Priority: json.Number("1"),
				},
			},
		},
	}, nil
}

func handleReadResourceRequest(ctx context.Context, req mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []mcp.Content{
			mcp.TextResourceContents{
				Text: "This is sample company data from Roodhals MCP server.",
				ResourceContents: mcp.ResourceContents{
					URI:      req.URI,
					MimeType: "text/plain",
				},
			},
		},
	}, nil
}

func handleListResourceTemplatesRequest(ctx context.Context, req mcp.ListResourceTemplatesParams) (*mcp.ListResourceTemplatesResult, error) {
	return &mcp.ListResourceTemplatesResult{
		ResourceTemplates: []mcp.ResourceTemplate{
			{
				Name:        "Roodhals Resources",
				Description: "Access Roodhals system resources.",
				URITemplate: "roodhals://resource/{path}",
			},
		},
	}, nil
}

func handleListPromptsRequest(ctx context.Context, req mcp.ListPromptsParams) (*mcp.ListPromptsResult, error) {
	return &mcp.ListPromptsResult{
		Prompts: []mcp.Prompt{
			{
				Name:        "analyze_data",
				Description: "Analyze data and provide insights",
				Arguments: []mcp.PromptArgument{
					{
						Name:        "data",
						Description: "The data to analyze",
						Required:    true,
					},
				},
			},
		},
	}, nil
}

func handleCompleteRequest(ctx context.Context, req mcp.CompleteParams) (*mcp.CompleteResult, error) {
	return &mcp.CompleteResult{
		Completion: mcp.Completion{
			Values: []string{},
		},
	}, nil
}

func handleGetPromptRequest(ctx context.Context, req mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	switch req.Name {
	case "analyze_data":
		data, ok := req.Arguments["data"]
		if !ok {
			return nil, errors.New("data argument is required")
		}
		return &mcp.GetPromptResult{
			Description: "Data analysis prompt",
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Text: "Please analyze this data and provide insights:\n" + data,
					},
				},
			},
		}, nil
	}
	return nil, errors.New("prompt not found")
}
