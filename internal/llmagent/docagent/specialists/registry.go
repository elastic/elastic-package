// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package specialists

import (
	"fmt"
	"sync"
)

// Registry manages available section agents and their execution order.
// It provides a dynamic way to add, remove, and retrieve agents for workflows.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]SectionAgent
	order  []string // maintains registration order for sequential execution
}

// NewRegistry creates a new empty agent registry.
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]SectionAgent),
		order:  make([]string, 0),
	}
}

// Register adds an agent to the registry.
// If an agent with the same name already exists, it will be replaced.
func (r *Registry) Register(agent SectionAgent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := agent.Name()
	if _, exists := r.agents[name]; !exists {
		r.order = append(r.order, name)
	}
	r.agents[name] = agent
}

// Unregister removes an agent from the registry by name.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[name]; exists {
		delete(r.agents, name)
		// Remove from order slice
		for i, n := range r.order {
			if n == name {
				r.order = append(r.order[:i], r.order[i+1:]...)
				break
			}
		}
	}
}

// Get retrieves an agent by name.
func (r *Registry) Get(name string) (SectionAgent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[name]
	return agent, exists
}

// All returns all registered agents in registration order.
func (r *Registry) All() []SectionAgent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]SectionAgent, 0, len(r.order))
	for _, name := range r.order {
		if agent, exists := r.agents[name]; exists {
			agents = append(agents, agent)
		}
	}
	return agents
}

// Names returns the names of all registered agents in order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.order))
	copy(names, r.order)
	return names
}

// Count returns the number of registered agents.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// SetOrder updates the execution order of agents.
// Returns an error if any name in the new order is not registered.
func (r *Registry) SetOrder(order []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate all names exist
	for _, name := range order {
		if _, exists := r.agents[name]; !exists {
			return fmt.Errorf("agent %q not found in registry", name)
		}
	}

	r.order = make([]string, len(order))
	copy(r.order, order)
	return nil
}

// DefaultRegistry creates a registry with the default set of documentation agents.
// This includes: Generator, Critic, Validator, and URLValidator.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(NewGeneratorAgent())
	r.Register(NewCriticAgent())
	r.Register(NewValidatorAgent())
	r.Register(NewURLValidatorAgent())
	return r
}
