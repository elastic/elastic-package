// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package mcptools

import (
	"encoding/json"
	"io"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"

	"github.com/elastic/elastic-package/internal/configuration/locations"
)

// MCPServer represents a Model Context Protocol server configuration.
// It can connect to either a local process or a remote URL endpoint.
type MCPServer struct {
	Command *string            `json:"command"`
	Args    []string           `json:"args"`
	Env     *map[string]string `json:"env"`
	Url     *string            `json:"url"`
	Headers *map[string]string `json:"headers"`

	Toolset tool.Toolset `json:"-"`
}

// MCPJson represents the MCP configuration file structure.
type MCPJson struct {
	InitialPrompt  *string              `json:"initialPromptFile"`
	RevisionPrompt *string              `json:"revisionPromptFile"`
	Servers        map[string]MCPServer `json:"mcpServers"`
}

// Connect establishes a connection to the MCP server using ADK's mcptoolset.
// It returns an error if the connection fails.
func (s *MCPServer) Connect() error {
	if s.Url == nil {
		return nil // Skip servers without URL
	}

	// Create a streamable transport for the MCP server
	transport := &mcp.StreamableClientTransport{Endpoint: *s.Url}

	// Create the MCP toolset using ADK
	toolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: transport,
	})
	if err != nil {
		return err
	}

	s.Toolset = toolset
	return nil
}

// Close terminates the MCP server connection.
func (s *MCPServer) Close() error {
	s.Toolset = nil
	return nil
}

// LoadToolsets loads MCP server configurations from the elastic-package config directory
// and returns ADK toolsets for all configured servers. It returns nil if the
// configuration file doesn't exist or if there are errors loading it.
func LoadToolsets() []tool.Toolset {
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

	var toolsets []tool.Toolset

	// Connect to all configured servers and collect their toolsets
	for key, value := range mcpJson.Servers {
		if value.Url != nil {
			if err := value.Connect(); err != nil {
				log.Printf("failed to connect to MCP server %s: %v", key, err)
				continue
			}
			if value.Toolset != nil {
				toolsets = append(toolsets, value.Toolset)
			}
		}
	}

	return toolsets
}
