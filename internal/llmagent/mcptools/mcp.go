// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package mcptools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/llmagent/providers"
)

const (
	// toolCallTimeout is the maximum time allowed for an MCP tool call
	toolCallTimeout = 30 * time.Second
)

// MCPServer represents a Model Context Protocol server configuration.
// It can connect to either a local process or a remote URL endpoint.
type MCPServer struct {
	Command *string            `json:"command"`
	Args    []string           `json:"args"`
	Env     *map[string]string `json:"env"`
	Url     *string            `json:"url"`
	Headers *map[string]string `json:"headers"`

	session *mcp.ClientSession `json:"-"`
	Tools   []providers.Tool   `json:"-"`
}

// MCPJson represents the MCP configuration file structure.
type MCPJson struct {
	InitialPrompt  *string              `json:"initialPromptFile"`
	RevisionPrompt *string              `json:"revisionPromptFile"`
	Servers        map[string]MCPServer `json:"mcpServers"`
}

// Connect establishes a connection to the MCP server and loads available tools.
// It returns an error if the connection fails or if tool loading fails.
func (s *MCPServer) Connect() error {
	if s.Url == nil {
		return fmt.Errorf("URL is required for MCP server connection")
	}

	ctx := context.Background()
	transport := &mcp.StreamableClientTransport{Endpoint: *s.Url}

	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)

	fmt.Printf("attempting to connect to %s\n", *s.Url)

	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	s.session = cs

	// Load tools if the server supports them
	if s.session.InitializeResult().Capabilities.Tools != nil {
		for tool, err := range s.session.Tools(ctx, nil) {
			if err != nil {
				log.Printf("failed to load tool: %v", err)
				continue
			}

			// Safely extract schema properties
			schema, ok := tool.InputSchema.(map[string]interface{})
			if !ok {
				log.Printf("unexpected InputSchema type for tool %s, skipping", tool.Name)
				continue
			}

			required := schema["required"]
			if required == nil {
				required = []string{}
			}

			properties := schema["properties"]

			// Capture tool name to avoid closure bug
			toolName := tool.Name

			s.Tools = append(s.Tools, providers.Tool{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": properties,
					"required":   required,
				},
				Handler: func(ctx context.Context, arguments string) (*providers.ToolResult, error) {
					callCtx, cancel := context.WithTimeout(ctx, toolCallTimeout)
					defer cancel()

					res, err := s.session.CallTool(callCtx, &mcp.CallToolParams{
						Name:      toolName,
						Arguments: json.RawMessage(arguments),
					})
					if err != nil {
						return nil, fmt.Errorf("failed to call tool %s: %w", toolName, err)
					}

					data, err := json.Marshal(res)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal tool result: %w", err)
					}

					return &providers.ToolResult{Content: string(data)}, nil
				},
			})
		}
	}

	return nil
}

// Close terminates the MCP server session if it exists.
func (s *MCPServer) Close() error {
	if s.session != nil {
		// The MCP SDK doesn't expose a Close method directly,
		// but we can clear the session reference
		s.session = nil
	}
	return nil
}

// LoadTools loads MCP server configurations from the elastic-package config directory
// and establishes connections to all configured servers. It returns nil if the
// configuration file doesn't exist or if there are errors loading it.
func LoadTools() *MCPJson {
	lm, err := locations.NewLocationManager()
	if err != nil {
		log.Printf("failed to create location manager: %v", err)
		return nil
	}

	mcpFile, err := os.Open(lm.MCPJson())
	if err != nil {
		// File not existing is expected in many cases, so no log needed
		return nil
	}
	defer mcpFile.Close()

	byteValue, err := io.ReadAll(mcpFile)
	if err != nil {
		log.Printf("failed to read MCP config file: %v", err)
		return nil
	}

	var mcpJson MCPJson
	if err := json.Unmarshal(byteValue, &mcpJson); err != nil {
		log.Printf("failed to unmarshal MCP config: %v", err)
		return nil
	}

	// Connect to all configured servers
	for key, value := range mcpJson.Servers {
		if value.Url != nil {
			if err := value.Connect(); err != nil {
				log.Printf("failed to connect to MCP server %s: %v", key, err)
				continue
			}
			mcpJson.Servers[key] = value
		}
	}

	return &mcpJson
}
