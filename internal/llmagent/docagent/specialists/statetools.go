// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package specialists

import (
	"encoding/json"
	"fmt"
	"sync"
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
var (
	activeStateStore *StateStore
	activeStateMu    sync.RWMutex
)

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

// valueToString converts a state value to a string for external consumption
func valueToString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case bool:
		return fmt.Sprintf("%v", v)
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(jsonBytes)
	}
}

// GetString retrieves a string value from state
func (s *StateStore) GetString(key string) (string, bool) {
	value, ok := s.Get(key)
	if !ok {
		return "", false
	}
	return valueToString(value), true
}
