// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists"
	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/logger"
)

// Builder constructs multi-agent workflows for documentation generation
type Builder struct {
	config Config
}

// NewBuilder creates a new workflow builder with the given configuration
func NewBuilder(cfg Config) *Builder {
	if cfg.Registry == nil {
		cfg.Registry = specialists.DefaultRegistry()
	}
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = DefaultMaxIterations
	}
	return &Builder{config: cfg}
}

// Result holds the output of a workflow execution
type Result struct {
	// Content is the final generated content
	Content string
	// Approved indicates if the content passed all checks
	Approved bool
	// Iterations is the number of refinement cycles
	Iterations int
	// Feedback contains the final feedback (if any)
	Feedback string
	// ValidationResult contains validation results
	ValidationResult *specialists.ValidationResult
	// URLCheckResult contains URL check results
	URLCheckResult *specialists.URLCheckResult
}

// BuildSectionWorkflow creates a workflow agent for generating a single section
func (b *Builder) BuildSectionWorkflow(ctx context.Context) (agent.Agent, error) {
	// Build sub-agents from registry
	subAgents, err := b.buildSubAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build sub-agents: %w", err)
	}

	if len(subAgents) == 0 {
		return nil, fmt.Errorf("no agents registered in workflow")
	}

	// If only one agent, return it directly
	if len(subAgents) == 1 {
		return subAgents[0], nil
	}

	// Create a sequential agent to run all agents in order
	seqAgent, err := sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:        "section-pipeline",
			Description: "Runs documentation agents in sequence",
			SubAgents:   subAgents,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create sequential agent: %w", err)
	}

	// Wrap in loop agent for iterative refinement
	if b.config.MaxIterations > 1 {
		return loopagent.New(loopagent.Config{
			AgentConfig: agent.Config{
				Name:        "section-workflow",
				Description: "Iteratively generates and refines documentation sections",
				SubAgents:   []agent.Agent{seqAgent},
			},
			MaxIterations: b.config.MaxIterations,
		})
	}

	return seqAgent, nil
}

// buildSubAgents creates ADK agents from the registry
func (b *Builder) buildSubAgents(ctx context.Context) ([]agent.Agent, error) {
	agentCfg := specialists.AgentConfig{
		Model:    b.config.Model,
		Tools:    b.config.Tools,
		Toolsets: b.config.Toolsets,
	}

	var subAgents []agent.Agent
	for _, sa := range b.config.Registry.All() {
		// Skip disabled agents
		switch sa.Name() {
		case "critic":
			if !b.config.EnableCritic {
				continue
			}
		case "validator":
			if !b.config.EnableValidator {
				continue
			}
		case "url_validator":
			if !b.config.EnableURLValidator {
				continue
			}
		}

		adkAgent, err := sa.Build(ctx, agentCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to build agent %s: %w", sa.Name(), err)
		}
		subAgents = append(subAgents, adkAgent)
	}

	return subAgents, nil
}

// ExecuteWorkflow runs the workflow and returns the result
func (b *Builder) ExecuteWorkflow(ctx context.Context, sectionCtx specialists.SectionContext) (*Result, error) {
	// Start workflow span for tracing with configuration
	ctx, span := tracing.StartWorkflowSpanWithConfig(ctx, "workflow:section", b.config.MaxIterations)
	defer func() {
		span.End()
	}()

	// Build the workflow agent
	workflowAgent, err := b.BuildSectionWorkflow(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build workflow: %w", err)
	}

	// Serialize section context for initial state
	ctxJSON, err := json.Marshal(sectionCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize section context: %w", err)
	}

	// Create session service with initial state
	sessionService := session.InMemoryService()

	// Create runner
	r, err := runner.New(runner.Config{
		AppName:        "docagent-workflow",
		Agent:          workflowAgent,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	// Create session with initial state
	sess, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: "docagent-workflow",
		UserID:  "docagent",
		State: map[string]any{
			specialists.StateKeySectionContext: string(ctxJSON),
			specialists.StateKeyIteration:      0,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Create initial user content
	userContent := genai.NewContentFromText(
		fmt.Sprintf("Generate documentation for section: %s", sectionCtx.SectionTitle),
		genai.RoleUser,
	)

	// Run the workflow
	result := &Result{}
	iterations := 0

	for event, err := range r.Run(ctx, "docagent", sess.Session.ID(), userContent, agent.RunConfig{}) {
		if err != nil {
			logger.Debugf("Workflow error: %v", err)
			return nil, fmt.Errorf("workflow execution error: %w", err)
		}

		if event == nil {
			continue
		}

		// Process state updates from events
		if event.Actions.StateDelta != nil {
			if content, ok := event.Actions.StateDelta[specialists.StateKeyContent].(string); ok {
				result.Content = content
			}
			if approved, ok := event.Actions.StateDelta[specialists.StateKeyApproved].(bool); ok {
				result.Approved = approved
			}
			if feedback, ok := event.Actions.StateDelta[specialists.StateKeyFeedback].(string); ok {
				result.Feedback = feedback
			}
			if vr, ok := event.Actions.StateDelta[specialists.StateKeyValidation].(specialists.ValidationResult); ok {
				result.ValidationResult = &vr
			}
			if ur, ok := event.Actions.StateDelta[specialists.StateKeyURLCheck].(specialists.URLCheckResult); ok {
				result.URLCheckResult = &ur
			}
		}

		// Track iterations (each loop is a complete iteration)
		if event.Author == "section-workflow" || event.Author == "section-pipeline" {
			iterations++
		}

		// Check for final response
		if event.IsFinalResponse() {
			break
		}

		// Check if approved early
		if result.Approved {
			logger.Debugf("Workflow completed early: content approved at iteration %d", iterations)
			break
		}
	}

	result.Iterations = iterations

	// Record workflow result for tracing
	tracing.RecordWorkflowResult(span, result.Approved, result.Iterations, result.Content)

	return result, nil
}
