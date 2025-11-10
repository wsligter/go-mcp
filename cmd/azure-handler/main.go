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
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/dstotijn/go-mcp"
)

const (
	serverName    = "mcp-azure-server"
	serverVersion = "0.1.0"
)

func main() {
	// Get port from environment variable (Azure Functions sets this)
	customHandlerPort, exists := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if !exists {
		customHandlerPort = "8080"
	}

	ctx := context.Background()

	// Configure SSE transport for Azure Functions
	host := "localhost"
	port := customHandlerPort

	sseURL := url.URL{
		Scheme: "http",
		Host:   host + ":" + port,
	}

	mcpServer := mcp.NewServer(mcp.ServerConfig{
		Name:                    serverName,
		Version:                 serverVersion,
		ListResourcesFn:         handleListResourcesRequest,
		ReadResourceFn:          handleReadResourceRequest,
		ListResourceTemplatesFn: handleListResourceTemplatesRequest,
		ListPromptsFn:           handleListPromptsRequest,
		GetPromptFn:             handleGetPromptRequest,
		CompleteFn:              handleCompleteRequest,
	}, mcp.WithSSETransport(sseURL))

	mcpServer.Start(ctx)

	// Register example tool
	type getWeatherArgs struct {
		Location string `json:"location"`
	}

	mcpServer.RegisterTools(mcp.CreateTool(mcp.ToolDef[getWeatherArgs]{
		Name:        "get_weather",
		Description: "Get current weather information for a location",
		HandleFunc: func(ctx context.Context, args getWeatherArgs) *mcp.CallToolResult {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Text: fmt.Sprintf("The weather in %v is sunny.", args.Location),
					},
				},
			}
		},
	}))

	httpServer := &http.Server{
		Addr:    ":" + customHandlerPort,
		Handler: mcpServer,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	log.Printf("MCP server started (%q, %q) on port %s", serverName, serverVersion, customHandlerPort)
	log.Printf("SSE transport endpoint: %v", sseURL.String())

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func handleListResourcesRequest(ctx context.Context, req mcp.ListResourcesParams) (*mcp.ListResourcesResult, error) {
	return &mcp.ListResourcesResult{
		Resources: []mcp.Resource{
			{
				Name:        "Azure Resource",
				URI:         "azure://resource/example",
				MimeType:    "text/plain",
				Description: "An example resource from Azure Functions.",
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
				Text: "Hello from Azure Functions!",
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
				Name:        "Azure Resources",
				Description: "Access Azure resources.",
				URITemplate: "azure://resource/{path}",
			},
		},
	}, nil
}

func handleListPromptsRequest(ctx context.Context, req mcp.ListPromptsParams) (*mcp.ListPromptsResult, error) {
	return &mcp.ListPromptsResult{
		Prompts: []mcp.Prompt{
			{
				Name:        "code_review",
				Description: "Asks the LLM to analyze code quality and suggest improvements",
				Arguments: []mcp.PromptArgument{
					{
						Name:        "code",
						Description: "The code to review",
						Required:    true,
					},
				},
			},
		},
	}, nil
}

func handleCompleteRequest(ctx context.Context, req mcp.CompleteParams) (*mcp.CompleteResult, error) {
	_, ok := req.Ref.(mcp.ResourceReference)
	if !ok || req.Argument.Name != "path" {
		return &mcp.CompleteResult{
			Completion: mcp.Completion{
				Values: []string{},
			},
		}, nil
	}

	path1 := "foo/bar/baz"
	path2 := "foo/bar/qux"

	if req.Argument.Value == "" {
		return &mcp.CompleteResult{
			Completion: mcp.Completion{
				Values: []string{path1, path2},
			},
		}, nil
	}

	values := []string{}
	if strings.Contains(path1, strings.ToLower(req.Argument.Value)) {
		values = append(values, path1)
	}
	if strings.Contains(path2, strings.ToLower(req.Argument.Value)) {
		values = append(values, path2)
	}

	return &mcp.CompleteResult{
		Completion: mcp.Completion{
			Values: values,
		},
	}, nil
}

func handleGetPromptRequest(ctx context.Context, req mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	switch req.Name {
	case "code_review":
		code, ok := req.Arguments["code"]
		if !ok {
			return nil, errors.New("code argument is required")
		}
		return &mcp.GetPromptResult{
			Description: "Code review prompt",
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Text: "Please review this code:\n" + code,
					},
				},
			},
		}, nil
	}
	return nil, errors.New("prompt not found")
}
