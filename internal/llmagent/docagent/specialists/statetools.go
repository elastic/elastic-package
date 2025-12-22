// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package specialists

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// StateStore provides access to session state for tools.
// This is used by the workflow to share state between agents.
type StateStore struct {
	mu    sync.RWMutex
	state map[string]any
}

// NewStateStore creates a new state store with initial state
func NewStateStore(initial map[string]any) *StateStore {
	if initial == nil {
		initial = make(map[string]any)
	}
	return &StateStore{state: initial}
}

// Get retrieves a value from state
func (s *StateStore) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.state[key]
	return val, ok
}

// Set stores a value in state
func (s *StateStore) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[key] = value
}

// All returns all state
func (s *StateStore) All() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]any, len(s.state))
	for k, v := range s.state {
		result[k] = v
	}
	return result
}

// activeStateStore is the current state store used by state tools.
// This is set by the workflow before running agents.
// Thread-safe access is managed by the StateStore itself.
var activeStateStore *StateStore
var activeStateMu sync.RWMutex

// SetActiveStateStore sets the state store for the current workflow execution.
// This must be called before running agents that use state tools.
func SetActiveStateStore(store *StateStore) {
	activeStateMu.Lock()
	defer activeStateMu.Unlock()
	activeStateStore = store
}

// GetActiveStateStore returns the current active state store.
func GetActiveStateStore() *StateStore {
	activeStateMu.RLock()
	defer activeStateMu.RUnlock()
	return activeStateStore
}

// ClearActiveStateStore clears the active state store.
func ClearActiveStateStore() {
	activeStateMu.Lock()
	defer activeStateMu.Unlock()
	activeStateStore = nil
}

// stateContextKey is used to store StateStore in context
type stateContextKey struct{}

// ContextWithState returns a context with the state store attached
func ContextWithState(ctx context.Context, store *StateStore) context.Context {
	return context.WithValue(ctx, stateContextKey{}, store)
}

// StateFromContext retrieves the state store from context
func StateFromContext(ctx context.Context) *StateStore {
	store, _ := ctx.Value(stateContextKey{}).(*StateStore)
	return store
}

// ReadStateArgs represents arguments for read_state tool
type ReadStateArgs struct {
	Key string `json:"key"`
}

// ReadStateResult represents the result of read_state tool
type ReadStateResult struct {
	Found bool   `json:"found"`
	Value string `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
}

// WriteStateArgs represents arguments for write_state tool
type WriteStateArgs struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// WriteStateResult represents the result of write_state tool
type WriteStateResult struct {
	Success bool   `json:"success"`
	Key     string `json:"key,omitempty"`
	Error   string `json:"error,omitempty"`
}

// normalizeKey ensures the key has the temp: prefix
func normalizeKey(key string) string {
	if len(key) >= 5 && key[:5] == "temp:" {
		return key
	}
	return "temp:" + key
}

// readStateHandler creates a handler for the read_state tool
func readStateHandler() functiontool.Func[ReadStateArgs, ReadStateResult] {
	return func(_ tool.Context, args ReadStateArgs) (ReadStateResult, error) {
		store := GetActiveStateStore()
		if store == nil {
			return ReadStateResult{Error: "no state store available"}, nil
		}

		fullKey := normalizeKey(args.Key)
		value, exists := store.Get(fullKey)
		if !exists {
			// Try without prefix
			value, exists = store.Get(args.Key)
			if !exists {
				return ReadStateResult{Found: false}, nil
			}
		}

		// Convert to string for LLM consumption
		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = v
		case []byte:
			valueStr = string(v)
		case bool:
			valueStr = fmt.Sprintf("%v", v)
		default:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				valueStr = fmt.Sprintf("%v", v)
			} else {
				valueStr = string(jsonBytes)
			}
		}

		return ReadStateResult{Found: true, Value: valueStr}, nil
	}
}

// writeStateHandler creates a handler for the write_state tool
func writeStateHandler() functiontool.Func[WriteStateArgs, WriteStateResult] {
	return func(_ tool.Context, args WriteStateArgs) (WriteStateResult, error) {
		store := GetActiveStateStore()
		if store == nil {
			return WriteStateResult{Error: "no state store available"}, nil
		}

		fullKey := normalizeKey(args.Key)

		// For approved key, convert to boolean
		if args.Key == "approved" || fullKey == "temp:approved" {
			boolVal := args.Value == "true" || args.Value == "1" || args.Value == "yes"
			store.Set(fullKey, boolVal)
		} else {
			store.Set(fullKey, args.Value)
		}

		return WriteStateResult{Success: true, Key: fullKey}, nil
	}
}

// StateTools returns the tools needed for state access
func StateTools() []tool.Tool {
	readStateTool, err := functiontool.New(
		functiontool.Config{
			Name:        "read_state",
			Description: "Read a value from the workflow session state. Use this to retrieve content written by other agents (e.g., section_content, feedback, validation_result).",
		},
		readStateHandler(),
	)
	if err != nil {
		panic("failed to create read_state tool: " + err.Error())
	}

	writeStateTool, err := functiontool.New(
		functiontool.Config{
			Name:        "write_state",
			Description: "Write a value to the workflow session state. Use this to store content for other agents to read (e.g., section_content after generating, feedback after reviewing, approved status).",
		},
		writeStateHandler(),
	)
	if err != nil {
		panic("failed to create write_state tool: " + err.Error())
	}

	return []tool.Tool{readStateTool, writeStateTool}
}
