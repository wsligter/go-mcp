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
	"sort"
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

	log.Printf("MCP Request: method=%s, id=%v, supportsSSE=%v", req.Method, req.ID, supportsSSE)

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

	// Per Streamable HTTP spec: server MAY return either JSON or SSE
	// For simplicity and stateless design, we use JSON for all responses
	// SSE is only needed for streaming/incremental responses
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSSEStream handles GET requests for SSE streams
// Per Streamable HTTP spec, server MAY support GET for server-initiated messages
func handleSSEStream(w http.ResponseWriter, r *http.Request) {
	// Check if client accepts SSE
	acceptHeader := r.Header.Get("Accept")
	if !strings.Contains(acceptHeader, "text/event-stream") {
		http.Error(w, "This endpoint requires Accept: text/event-stream for GET requests", http.StatusNotAcceptable)
		return
	}

	log.Printf("SSE stream requested via GET")

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// For stateless design, we don't maintain long-lived SSE connections
	// Just return a comment and close
	// A full implementation would keep the connection open and send notifications
	fmt.Fprintf(w, ": MCP server ready (stateless mode - no persistent SSE)\n\n")

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	log.Printf("SSE stream closed (stateless mode)")
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
		"instructions": `CBS (Statistics Netherlands) Open Data API Access

WORKFLOW FOR QUERYING CBS DATA:
1. SEARCH: Use query_datasets with search parameter to find relevant datasets (e.g., search="population" or search="bevolking")
2. FILTER: Always add filter="Status ne 'Gediscontinueerd'" to exclude discontinued datasets
3. IDENTIFY: Note the dataset ID (e.g., "83765NED") from the results
4. EXPLORE: Use get_dimensions with the dataset ID to see available dimensions (time periods, regions, etc.)
5. QUERY: Use query_observations or get_observations with appropriate filters to get the actual data

IMPORTANT TIPS:
- Dataset titles are in Dutch. Common terms: "Bevolking" (population), "Economie" (economy), "Arbeidsmarkt" (labor market)
- Use search parameter for free-text search across dataset titles and descriptions
- Time dimensions are usually named "Perioden" (periods) with format like "2022JJ00" (year 2022)
- Regional dimensions are usually "RegioS" (regions) with codes like "NL01" (Netherlands)
- Always check dimensions first to understand available filters before querying observations

EXAMPLE: To find 2022 Netherlands population:
1. query_datasets(catalog="CBS", search="bevolking", filter="Status ne 'Gediscontinueerd'", top=5)
2. get_dimensions(catalog="CBS", dataset="<ID from step 1>")
3. query_observations(catalog="CBS", dataset="<ID>", filter="Perioden eq '2022JJ00' and RegioS eq 'NL01'")`,
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
				"description": "Search and list CBS datasets. ALWAYS use search parameter (e.g., search='bevolking' for population, search='economie' for economy). ALWAYS include filter='Status ne Gediscontinueerd' to exclude discontinued datasets. Returns dataset IDs needed for other tools.",
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
				"description": "REQUIRED BEFORE QUERYING DATA: Get all dimensions (Perioden=time, RegioS=regions, etc.) for a dataset. Use this to understand what filters are available. Returns dimension keys and codes needed for query_observations filters.",
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
				"description": "Get actual statistical data from a dataset. Use OData filters based on dimensions from get_dimensions. Example filters: Perioden eq '2022JJ00' (year 2022), RegioS eq 'NL01' (Netherlands), combine with 'and'. Always call get_dimensions first to see available filter values.",
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
			{
				"name":        "get_observations",
				"description": "Retrieve observations from a specific CBS dataset with optional filters",
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
						"filters": map[string]interface{}{
							"type":        "object",
							"description": "Optional filters as key-value pairs",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of observations to return",
						},
					},
					"required": []string{"catalog", "dataset"},
				},
			},
			{
				"name":        "get_metadata",
				"description": "Retrieves the OData metadata document describing the CBS API structure",
				"inputSchema": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			{
				"name":        "get_dimension_values",
				"description": "Retrieves all possible values for a specific dimension in a dataset",
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
						"dimension": map[string]interface{}{
							"type":        "string",
							"description": "Dimension identifier",
						},
					},
					"required": []string{"catalog", "dataset", "dimension"},
				},
			},
			{
				"name":        "resolve_dimension_value",
				"description": "Resolve a natural language input (e.g., 'Netherlands', '2019', 'total') to concrete CBS dimension codes for a given dataset and dimension. Use this after get_dimensions to translate user-friendly values into the exact codes needed in OData filters for query_observations or get_observations.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"catalog": map[string]interface{}{
							"type":        "string",
							"description": "Catalog identifier (use 'CBS')",
						},
						"dataset": map[string]interface{}{
							"type":        "string",
							"description": "Dataset identifier (e.g., '03759ned')",
						},
						"dimension": map[string]interface{}{
							"type":        "string",
							"description": "Dimension identifier (e.g., 'RegioS', 'Perioden', 'Geslacht')",
						},
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Natural language or simple value to resolve (e.g., 'Netherlands', '2019', 'total')",
						},
						"maxCandidates": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of candidate codes to return (default: 5)",
						},
					},
					"required": []string{"catalog", "dataset", "dimension", "query"},
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
			log.Printf("query_datasets failed for catalog=%s options=%v: %v", catalog, queryOpts, err)
			return successResponse(fmt.Sprintf("Unable to query datasets for catalog '%s' with the given search/filter options. This usually means the combination of search, filter, top/skip is invalid or the CBS API returned an error. Try simplifying the query: remove complex filters, reduce 'top', or search with a simpler term (for example, search='bevolking' and filter='Status ne 'Gediscontinueerd'').\n\nDetails: %v", catalog, err))
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
			log.Printf("get_dimensions failed for catalog=%s dataset=%s: %v", catalog, dataset, err)
			return successResponse(fmt.Sprintf("Unable to retrieve dimensions for dataset '%s' in catalog '%s'. This usually means the dataset identifier is invalid or the CBS API returned an error. Verify the dataset ID from query_datasets and try again.\n\nDetails: %v", dataset, catalog, err))
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
			log.Printf("query_observations failed for path=%s options=%v: %v", path, queryOpts, err)
			return successResponse(fmt.Sprintf("Unable to query observations for dataset '%s'. This usually means the OData filter or select is invalid (for example, using a code that does not exist in this dataset). Use get_dimensions and resolve_dimension_value (or get_dimension_values) to discover valid dimension codes, then try a simpler filter such as:\n- Perioden eq 'VALID_PERIOD_CODE' and RegioS eq 'VALID_REGION_CODE'\n\nDetails: %v", dataset, err))
		}

		limit := 100
		if top, ok := callParams.Arguments["top"].(float64); ok && top > 0 {
			limit = int(top)
		}

		formatted, err := FormatObservations(result, limit)
		if err != nil {
			log.Printf("query_observations formatting failed for dataset=%s: %v", dataset, err)
			return successResponse(fmt.Sprintf("Observations were retrieved but could not be formatted as JSON. You can retry with a smaller 'top' value or a simpler filter to reduce the response size.\n\nDetails: %v", err))
		}

		return successResponse(formatted)

	case "get_observations":
		catalog, _ := callParams.Arguments["catalog"].(string)
		dataset, _ := callParams.Arguments["dataset"].(string)

		if catalog == "" || dataset == "" {
			return errorResponse("catalog and dataset parameters are required")
		}

		// Extract filters if provided
		filters := make(map[string]string)
		if filtersArg, ok := callParams.Arguments["filters"].(map[string]interface{}); ok {
			for k, v := range filtersArg {
				if strVal, ok := v.(string); ok {
					filters[k] = strVal
				}
			}
		}

		observations, err := client.GetObservations(catalog, dataset, filters)
		if err != nil {
			log.Printf("get_observations failed for catalog=%s dataset=%s filters=%v: %v", catalog, dataset, filters, err)
			return successResponse(fmt.Sprintf("Unable to retrieve observations for dataset '%s' with the current filters. This usually means one or more filter values are invalid. Use get_dimensions together with resolve_dimension_value (or get_dimension_values) to find valid codes, then try again.\n\nDetails: %v", dataset, err))
		}

		// Apply limit if specified
		limit := len(observations)
		if limitArg, ok := callParams.Arguments["limit"].(float64); ok && limitArg > 0 {
			limit = int(limitArg)
			if limit > len(observations) {
				limit = len(observations)
			}
		}

		limitedObs := observations[:limit]
		obsJSON, _ := json.Marshal(limitedObs)

		return successResponse(fmt.Sprintf("Retrieved %d observations (showing %d):\n\n```json\n%s\n```",
			len(observations), limit, string(obsJSON)))

	case "get_metadata":
		metadata, err := client.GetMetadata()
		if err != nil {
			return errorResponse(fmt.Sprintf("Failed to get metadata: %v", err))
		}

		// Truncate if too long
		if len(metadata) > 5000 {
			metadata = metadata[:5000] + "\n\n... (truncated)"
		}

		return successResponse(fmt.Sprintf("CBS API Metadata:\n\n```xml\n%s\n```", metadata))

	case "get_dimension_values":
		catalog, _ := callParams.Arguments["catalog"].(string)
		dataset, _ := callParams.Arguments["dataset"].(string)
		dimension, _ := callParams.Arguments["dimension"].(string)

		if catalog == "" || dataset == "" || dimension == "" {
			return errorResponse("catalog, dataset, and dimension parameters are required")
		}

		queryOpts := make(map[string]string)
		values, err := client.GetDimensionValues(catalog, dataset, dimension, queryOpts)
		if err != nil {
			// Do not surface this as an error to the model. Some datasets/dimensions simply
			// do not expose a DimensionValues endpoint (CBS returns 404). In that case,
			// guide the model to use get_dimensions + query_observations / get_observations
			// instead of repeatedly calling this tool.
			log.Printf("get_dimension_values failed for catalog=%s dataset=%s dimension=%s: %v", catalog, dataset, dimension, err)
			guidance := fmt.Sprintf(
				"Dimension values could not be retrieved for dataset '%s' (dimension '%s'). This often means the CBS API does not expose a DimensionValues endpoint for this dimension. Instead, use get_dimensions to inspect available dimensions and then query_observations or get_observations with appropriate OData filters (for example, using codes like Perioden eq '2022JJ00' for year 2022 and RegioS eq 'NL01' for the Netherlands).",
				dataset,
				dimension,
			)
			return successResponse(guidance)
		}

		valuesJSON, _ := json.Marshal(values)
		return successResponse(fmt.Sprintf("Found %d values for dimension '%s':\n\n```json\n%s\n```",
			len(values), dimension, string(valuesJSON)))

	case "resolve_dimension_value":
		catalog, _ := callParams.Arguments["catalog"].(string)
		dataset, _ := callParams.Arguments["dataset"].(string)
		dimension, _ := callParams.Arguments["dimension"].(string)
		query, _ := callParams.Arguments["query"].(string)

		if catalog == "" || dataset == "" || dimension == "" || query == "" {
			return errorResponse("catalog, dataset, dimension, and query parameters are required")
		}

		maxCandidates := 5
		if maxArg, ok := callParams.Arguments["maxCandidates"].(float64); ok && maxArg > 0 {
			maxCandidates = int(maxArg)
		}

		// Fetch all dimension values via CBS API.
		values, err := client.GetDimensionValues(catalog, dataset, dimension, map[string]string{})
		if err != nil {
			log.Printf("resolve_dimension_value: failed to get dimension values for catalog=%s dataset=%s dimension=%s: %v", catalog, dataset, dimension, err)
			return successResponse(fmt.Sprintf("Dimension values could not be loaded for dataset '%s' (dimension '%s'). This often means the DimensionValues endpoint is not available for this dataset or CBS returned an error. Use get_dimension_values directly (if available) or fall back to get_dimensions plus manual inspection of sample observations to infer valid codes.\n\nDetails: %v", dataset, dimension, err))
		}

		q := strings.ToLower(strings.TrimSpace(query))
		if q == "" {
			return errorResponse("query parameter must not be empty")
		}

		// Build a set of search keys to support both English and Dutch terms.
		searchKeys := []string{q}
		switch q {
		case "netherlands", "the netherlands":
			searchKeys = append(searchKeys, "nederland", "nl")
		case "nederland":
			searchKeys = append(searchKeys, "netherlands", "nl")
		case "total", "all", "overall":
			searchKeys = append(searchKeys, "totaal")
		case "totaal":
			searchKeys = append(searchKeys, "total")
		case "male", "man", "men":
			searchKeys = append(searchKeys, "man", "mannen")
		case "female", "woman", "women":
			searchKeys = append(searchKeys, "vrouw", "vrouwen")
		case "netherlands total", "nederland totaal":
			searchKeys = append(searchKeys, "nederland", "netherlands", "totaal")
		}

		// Score each value based on how well it matches any of the search keys.
		type candidate struct {
			code  string
			label string
			score int
			raw   map[string]any
		}

		var candidates []candidate

		for _, val := range values {
			code := ""
			label := ""

			// Heuristic: common field names in CBS dimension value tables.
			if v, ok := val["Key"].(string); ok {
				code = v
			}
			if v, ok := val["Code"].(string); ok && code == "" {
				code = v
			}
			if v, ok := val["Identifier"].(string); ok && code == "" {
				code = v
			}
			if v, ok := val["Title"].(string); ok {
				label = v
			}
			if v, ok := val["Description"].(string); ok && label == "" {
				label = v
			}

			// Build a text blob for matching.
			combined := strings.ToLower(code + " " + label)
			if combined == " " {
				continue
			}

			// Simple scoring: exact, prefix, contains for any search key.
			score := 0
			for _, sk := range searchKeys {
				if combined == sk {
					score += 100
				}
				if strings.Contains(combined, sk) {
					score += 50
				}
				if strings.HasPrefix(combined, sk) {
					score += 20
				}
			}
			if score == 0 {
				continue
			}

			candidates = append(candidates, candidate{
				code:  code,
				label: label,
				score: score,
				raw:   val,
			})
		}

		if len(candidates) == 0 {
			return successResponse(fmt.Sprintf("No matching dimension values found for query '%s' in dimension '%s'. Use get_dimension_values to inspect all available codes and then construct an OData filter manually.", query, dimension))
		}

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].score > candidates[j].score
		})

		if len(candidates) > maxCandidates {
			candidates = candidates[:maxCandidates]
		}

		// Build a friendly response describing the best matches and how to use them.
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Resolved query '%s' for dimension '%s' in dataset '%s'. Top candidate codes:\n\n", query, dimension, dataset))
		for i, c := range candidates {
			b.WriteString(fmt.Sprintf("%d. Code: `%s`", i+1, c.code))
			if c.label != "" {
				b.WriteString(fmt.Sprintf(" – %s", c.label))
			}
			b.WriteString("\n")
		}
		b.WriteString("\nUse these codes in OData filters for query_observations or get_observations, for example:\n")
		b.WriteString(fmt.Sprintf("- %s eq '%s'\n", dimension, candidates[0].code))

		return successResponse(b.String())
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
