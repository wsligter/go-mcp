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
	"log"
	"net/http"
	"os"

	"github.com/dstotijn/go-mcp"
)

const (
	serverName    = "roodhals-mcp-server"
	serverVersion = "0.1.0"
)

func main() {
	// Get port from environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx := context.Background()

	// Create MCP server WITHOUT SSE transport (Copilot Studio uses HTTP POST)
	mcpServer := mcp.NewServer(mcp.ServerConfig{
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

	// Register example tool
	type getWeatherArgs struct {
		Location string `json:"location" jsonschema:"description=The location to get weather for"`
	}

	mcpServer.RegisterTools(mcp.CreateTool(mcp.ToolDef[getWeatherArgs]{
		Name:        "get_weather",
		Description: "Get current weather information for a location",
		HandleFunc: func(ctx context.Context, args getWeatherArgs) *mcp.CallToolResult {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Text: fmt.Sprintf("The weather in %v is sunny with a temperature of 22°C.", args.Location),
					},
				},
			}
		},
	}))

	// Create HTTP handler for Copilot Studio (Streamable transport)
	http.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Set headers for streaming response
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Transfer-Encoding", "chunked")

		// Handle the MCP request
		mcpServer.ServeHTTP(w, r)
	})

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
