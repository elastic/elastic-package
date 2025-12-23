// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel/trace"
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

	logger.Debugf("Building workflow agents (EnableCritic=%v, EnableValidator=%v, EnableURLValidator=%v)",
		b.config.EnableCritic, b.config.EnableValidator, b.config.EnableURLValidator)

	var subAgents []agent.Agent
	for _, sa := range b.config.Registry.All() {
		// Skip disabled agents
		switch sa.Name() {
		case "critic":
			if !b.config.EnableCritic {
				logger.Debugf("Skipping critic agent (disabled)")
				continue
			}
		case "validator":
			if !b.config.EnableValidator {
				logger.Debugf("Skipping validator agent (disabled)")
				continue
			}
		case "url_validator":
			if !b.config.EnableURLValidator {
				logger.Debugf("Skipping url_validator agent (disabled)")
				continue
			}
		}

		logger.Debugf("Building agent: %s", sa.Name())
		adkAgent, err := sa.Build(ctx, agentCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to build agent %s: %w", sa.Name(), err)
		}
		subAgents = append(subAgents, adkAgent)
	}

	logger.Debugf("Built %d workflow agents", len(subAgents))
	return subAgents, nil
}

// ExecuteWorkflow runs the workflow and returns the result
func (b *Builder) ExecuteWorkflow(ctx context.Context, sectionCtx specialists.SectionContext) (*Result, error) {
	// Start workflow span for tracing with configuration
	ctx, span := tracing.StartWorkflowSpanWithConfig(ctx, "workflow:section", b.config.MaxIterations)

	// Result is initialized here so defer can access it
	result := &Result{}
	iterations := 0

	defer func() {
		// Always record workflow result before ending span
		tracing.RecordWorkflowResult(span, result.Approved, iterations, result.Content)
		span.End()
	}()

	// Serialize section context for initial state
	ctxJSON, err := json.Marshal(sectionCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize section context: %w", err)
	}

	// Create state store with initial state - this is used by the state tools
	stateStore := specialists.NewStateStore(map[string]any{
		specialists.StateKeySectionContext: string(ctxJSON),
		specialists.StateKeyIteration:      0,
	})

	// Set the active state store for tools to access
	specialists.SetActiveStateStore(stateStore)
	defer specialists.ClearActiveStateStore()

	// Add state store to context (for any code that might need it directly)
	ctx = specialists.ContextWithState(ctx, stateStore)

	// Build the workflow agent (must be after context setup for tools to work)
	workflowAgent, err := b.BuildSectionWorkflow(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build workflow: %w", err)
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

	// Create session with initial state (mirrors our state store)
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
		fmt.Sprintf("Generate documentation for section: %s. Use the read_state and write_state tools to access context and store your output.", sectionCtx.SectionTitle),
		genai.RoleUser,
	)

	// Track active agent span to avoid duplicates and measure actual duration
	var currentAgentName string
	var currentAgentSpan trace.Span
	var agentPromptTokens, agentCompletionTokens int

	// Helper to end the current agent span
	endCurrentAgentSpan := func() {
		if currentAgentSpan != nil {
			// Record token counts if we have them
			if agentPromptTokens > 0 || agentCompletionTokens > 0 {
				tracing.EndLLMSpan(ctx, currentAgentSpan, nil, agentPromptTokens, agentCompletionTokens)
			} else {
				tracing.SetSpanOk(currentAgentSpan)
				currentAgentSpan.End()
			}
			currentAgentSpan = nil
			currentAgentName = ""
			agentPromptTokens = 0
			agentCompletionTokens = 0
		}
	}

	for event, err := range r.Run(ctx, "docagent", sess.Session.ID(), userContent, agent.RunConfig{}) {
		if err != nil {
			logger.Debugf("Workflow error: %v", err)
			endCurrentAgentSpan() // Clean up on error
			return nil, fmt.Errorf("workflow execution error: %w", err)
		}

		if event == nil {
			continue
		}

		// Track agent transitions - start a new span when agent changes
		if event.Author != "" && event.Author != currentAgentName {
			// End previous agent span if any
			endCurrentAgentSpan()

			// Skip creating spans for workflow orchestrator agents
			if event.Author != "section-workflow" && event.Author != "section-pipeline" {
				logger.Debugf("Agent started: %s", event.Author)
				_, currentAgentSpan = tracing.StartAgentSpan(ctx, "agent:"+event.Author, b.config.ModelID)
				currentAgentName = event.Author
			}
		}

		// Accumulate token counts from events
		if event.UsageMetadata != nil {
			agentPromptTokens += int(event.UsageMetadata.PromptTokenCount)
			agentCompletionTokens += int(event.UsageMetadata.CandidatesTokenCount)
		}

		// Sync state from our store (updated by tools) to result
		if content, ok := stateStore.Get(specialists.StateKeyContent); ok {
			if contentStr, ok := content.(string); ok {
				result.Content = contentStr
			}
		}
		if approved, ok := stateStore.Get(specialists.StateKeyApproved); ok {
			if approvedBool, ok := approved.(bool); ok {
				result.Approved = approvedBool
			}
		}
		if feedback, ok := stateStore.Get(specialists.StateKeyFeedback); ok {
			if feedbackStr, ok := feedback.(string); ok {
				result.Feedback = feedbackStr
			}
		}

		// Also process state updates from ADK events (for URLValidator which uses session directly)
		if event.Actions.StateDelta != nil {
			if content, ok := event.Actions.StateDelta[specialists.StateKeyContent].(string); ok {
				result.Content = content
				stateStore.Set(specialists.StateKeyContent, content)
			}
			if approved, ok := event.Actions.StateDelta[specialists.StateKeyApproved].(bool); ok {
				result.Approved = approved
				stateStore.Set(specialists.StateKeyApproved, approved)
			}
			if feedback, ok := event.Actions.StateDelta[specialists.StateKeyFeedback].(string); ok {
				result.Feedback = feedback
				stateStore.Set(specialists.StateKeyFeedback, feedback)
			}
			if vrAny, ok := event.Actions.StateDelta[specialists.StateKeyValidation]; ok {
				if vr, ok := vrAny.(specialists.ValidationResult); ok {
					result.ValidationResult = &vr
				} else if vrPtr, ok := vrAny.(*specialists.ValidationResult); ok {
					result.ValidationResult = vrPtr
				}
			}
			if urAny, ok := event.Actions.StateDelta[specialists.StateKeyURLCheck]; ok {
				if ur, ok := urAny.(specialists.URLCheckResult); ok {
					result.URLCheckResult = &ur
				} else if urPtr, ok := urAny.(*specialists.URLCheckResult); ok {
					result.URLCheckResult = urPtr
				}
			}
		}

		// Track iterations (each loop is a complete iteration)
		if event.Author == "section-workflow" || event.Author == "section-pipeline" {
			iterations++
		}

		// Check for final response - only exit on final response from the outer workflow agent
		// Individual sub-agents (generator, critic, validator) will return final=true when they finish,
		// but we should continue to let the sequential agent run the next sub-agent
		if event.IsFinalResponse() {
			// End current agent span when it finishes
			if event.Author == currentAgentName {
				logger.Debugf("Agent finished: %s", event.Author)
				endCurrentAgentSpan()
			}

			// Only break if it's from the outer workflow (section-workflow or section-pipeline)
			// or if we've received final from an unknown/empty author (safety fallback)
			if event.Author == "section-workflow" || event.Author == "section-pipeline" || event.Author == "" {
				logger.Debugf("Final response from workflow %s, ending", event.Author)
				break
			}
			logger.Debugf("Final response from sub-agent %s, continuing workflow", event.Author)
		}

		// Check if approved early
		if result.Approved {
			logger.Debugf("Workflow completed early: content approved at iteration %d", iterations)
			endCurrentAgentSpan()
			break
		}
	}

	// Ensure any remaining span is closed
	endCurrentAgentSpan()

	result.Iterations = iterations
	return result, nil
}
