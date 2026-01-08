// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package workflow provides workflow orchestration for documentation generation.
// This file contains the shared context builder used by both `update documentation`
// and `test documentation` commands to ensure consistent document generation.
package workflow

import (
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
)

// ContextBuilder builds rich context for the documentation generator.
// This is the single source of truth for generator context - used by both
// `update documentation` (docagent) and `test documentation` (harness) commands.
type ContextBuilder struct {
	pkgCtx   *validators.PackageContext
	feedback []string
}

// NewContextBuilder creates a new context builder with the given package context
func NewContextBuilder(pkgCtx *validators.PackageContext) *ContextBuilder {
	return &ContextBuilder{
		pkgCtx: pkgCtx,
	}
}

// WithFeedback adds validation feedback to include in the context
func (b *ContextBuilder) WithFeedback(feedback []string) *ContextBuilder {
	b.feedback = feedback
	return b
}

// Build creates the complete context string for the generator
func (b *ContextBuilder) Build() string {
	if b.pkgCtx == nil || b.pkgCtx.Manifest == nil {
		return ""
	}

	var sb strings.Builder

	// REQUIRED DOCUMENT STRUCTURE - Always include this first
	sb.WriteString(b.buildRequiredStructure())

	// PACKAGE INFORMATION - Rich context about the package
	sb.WriteString(b.buildPackageInfo())

	// DATA STREAMS - What data the integration collects
	sb.WriteString(b.buildDataStreamsContext())

	// SERVICE INFO LINKS - Vendor documentation links
	sb.WriteString(b.buildServiceInfoLinks())

	// VENDOR SETUP INSTRUCTIONS - From service_info.md (if present)
	sb.WriteString(b.buildVendorSetupContext())

	// SERVICE INFO CONTENT - Raw content if no structured vendor setup
	sb.WriteString(b.buildServiceInfoContent())

	// ADVANCED SETTINGS - Configuration gotchas that must be documented
	sb.WriteString(b.buildAdvancedSettingsContext())

	// SCALING GUIDANCE - Input-specific performance and scaling recommendations
	sb.WriteString(b.buildScalingGuidance())

	// VALIDATION FEEDBACK - If there's feedback from previous iterations
	sb.WriteString(b.buildFeedbackContext())

	// INSTRUCTIONS - Final instructions for the generator
	sb.WriteString(b.buildInstructions())

	return sb.String()
}

// buildRequiredStructure returns the required document structure template
func (b *ContextBuilder) buildRequiredStructure() string {
	return fmt.Sprintf(`
REQUIRED DOCUMENT STRUCTURE (use these EXACT section names):
# %s

> **Note**: This documentation was generated using AI and should be reviewed for accuracy.

## Overview
### Compatibility
### How it works

## What data does this integration collect?
### Supported use cases

## What do I need to use this integration?

## How do I deploy this integration?
### Agent-based deployment
### Onboard and configure
### Validation

## Troubleshooting

## Performance and scaling

## Reference
### Inputs used
### API usage (if applicable)

`, b.pkgCtx.Manifest.Title)
}

// buildPackageInfo returns package metadata context
func (b *ContextBuilder) buildPackageInfo() string {
	var sb strings.Builder
	sb.WriteString("=== PACKAGE INFORMATION ===\n")
	sb.WriteString(fmt.Sprintf("Package Name: %s\n", b.pkgCtx.Manifest.Name))
	sb.WriteString(fmt.Sprintf("Package Title: %s\n", b.pkgCtx.Manifest.Title))
	if b.pkgCtx.Manifest.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", b.pkgCtx.Manifest.Description))
	}
	return sb.String()
}

// buildDataStreamsContext returns data streams documentation context
func (b *ContextBuilder) buildDataStreamsContext() string {
	if len(b.pkgCtx.DataStreams) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n=== DATA STREAMS (document ALL of these) ===\n")
	for _, ds := range b.pkgCtx.DataStreams {
		sb.WriteString(fmt.Sprintf("- %s", ds.Name))
		if ds.Title != "" && ds.Title != ds.Name {
			sb.WriteString(fmt.Sprintf(" (%s)", ds.Title))
		}
		if ds.Description != "" {
			sb.WriteString(fmt.Sprintf(": %s", ds.Description))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildServiceInfoLinks returns vendor documentation links context
func (b *ContextBuilder) buildServiceInfoLinks() string {
	if !b.pkgCtx.HasServiceInfoLinks() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n=== VENDOR DOCUMENTATION LINKS (MUST include ALL in documentation - use EXACT URLs) ===\n")
	sb.WriteString("IMPORTANT: Copy these URLs exactly as shown. Do NOT modify, shorten, or rephrase them.\n")
	for _, link := range b.pkgCtx.GetServiceInfoLinks() {
		sb.WriteString(fmt.Sprintf("- [%s](%s)\n", link.Text, link.URL))
	}
	return sb.String()
}

// buildVendorSetupContext returns vendor setup instructions from service_info.md
func (b *ContextBuilder) buildVendorSetupContext() string {
	if !b.pkgCtx.HasVendorSetupContent() {
		return ""
	}
	return "\n" + b.pkgCtx.GetVendorSetupForGenerator()
}

// buildServiceInfoContent returns raw service_info.md content (only if no structured vendor setup)
func (b *ContextBuilder) buildServiceInfoContent() string {
	// Only include raw service_info if we haven't already included the structured vendor setup
	if b.pkgCtx.ServiceInfo == "" || len(b.pkgCtx.ServiceInfo) >= 4000 || b.pkgCtx.HasVendorSetupContent() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n=== SERVICE INFO CONTENT (use this for context) ===\n")
	sb.WriteString(b.pkgCtx.ServiceInfo)
	sb.WriteString("\n")
	return sb.String()
}

// buildAdvancedSettingsContext returns advanced settings documentation context
func (b *ContextBuilder) buildAdvancedSettingsContext() string {
	return b.pkgCtx.FormatAdvancedSettingsForGenerator()
}

// buildFeedbackContext returns validation feedback context
func (b *ContextBuilder) buildFeedbackContext() string {
	if len(b.feedback) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n=== CRITICAL: VALIDATION ISSUES TO FIX ===\n")
	for _, f := range b.feedback {
		sb.WriteString(fmt.Sprintf("- %s\n", f))
	}
	return sb.String()
}

// buildInstructions returns the final instructions for the generator
func (b *ContextBuilder) buildInstructions() string {
	var sb strings.Builder
	sb.WriteString("\n=== INSTRUCTIONS ===\n")
	sb.WriteString("1. Use the EXACT section names shown above (## Overview, ## What data does this integration collect?, etc.)\n")
	sb.WriteString("2. Do NOT rename sections (e.g., don't use \"## Setup\" instead of \"## How do I deploy this integration?\")\n")
	sb.WriteString("3. IMMEDIATELY after the H1 title, add: \"> **Note**: This documentation was generated using AI and should be reviewed for accuracy.\"\n")
	sb.WriteString("4. Include ALL vendor documentation links - COPY URLS EXACTLY, do not modify them\n")
	sb.WriteString("5. Document ALL data streams listed above\n")
	sb.WriteString("6. Ensure heading hierarchy: # for title, ## for main sections, ### for subsections\n")
	sb.WriteString("7. Use {{event \"datastream\"}} and {{fields \"datastream\"}} placeholders in Reference section\n")
	sb.WriteString("8. Address EVERY validation issue if any are listed above\n")
	sb.WriteString("9. For code blocks, always specify the language (e.g., ```bash, ```yaml)\n")
	sb.WriteString("10. Document ALL advanced settings with appropriate warnings (security, debug, SSL, etc.)\n")
	return sb.String()
}

// buildScalingGuidance generates input-specific scaling guidance based on the package's inputs
func (b *ContextBuilder) buildScalingGuidance() string {
	if b.pkgCtx == nil || b.pkgCtx.Manifest == nil {
		return ""
	}

	// Extract unique input types from policy templates
	inputTypes := make(map[string]bool)
	for _, pt := range b.pkgCtx.Manifest.PolicyTemplates {
		for _, input := range pt.Inputs {
			if input.Type != "" {
				inputTypes[input.Type] = true
			}
		}
	}

	if len(inputTypes) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n=== PERFORMANCE AND SCALING GUIDANCE ===\n")
	sb.WriteString("Based on the inputs used by this integration, include the following guidance in the Performance and scaling section:\n\n")

	// Knowledge base of scaling guidance by input type
	scalingKnowledge := getScalingKnowledgeBase()

	for inputType := range inputTypes {
		if guidance, ok := scalingKnowledge[inputType]; ok {
			sb.WriteString(guidance)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// getScalingKnowledgeBase returns the knowledge base of scaling guidance by input type
// This is a shared resource used by both the context builder and validators
func getScalingKnowledgeBase() map[string]string {
	return map[string]string{
		"tcp": `**TCP/Syslog Input:**
- TCP provides guaranteed delivery with acknowledgments, making it suitable for production
- Configure multiple listeners on different ports for high availability
- Use a load balancer to distribute connections across multiple Elastic Agents
- TCP handles backpressure naturally - connections queue when Elasticsearch is slow`,

		"udp": `**UDP/Syslog Input (CRITICAL WARNING):**
- ⚠️ UDP does NOT guarantee delivery - data loss WILL occur during:
  - Network congestion
  - Agent restarts
  - Elasticsearch backpressure
- **Strongly recommend TCP for production systems requiring data integrity**
- If UDP is required: increase receive buffer size (SO_RCVBUF) for high-volume environments
- Consider multiple agents with DNS round-robin for redundancy`,

		"httpjson": `**HTTP JSON/API Polling Input:**
- Adjust polling interval to balance data freshness vs API load
- Configure request rate limiting to avoid overwhelming source APIs
- Be aware of vendor API rate limits and adjust accordingly
- Use pagination for large datasets to avoid timeouts
- Built-in retry with configurable exponential backoff handles transient failures`,

		"logfile": `**Log File Input:**
- Use glob patterns to monitor multiple log files efficiently
- Configure harvester_limit to control resource usage with many files
- Use close_inactive to release file handles for rotated logs
- File position tracking survives agent restarts - no data loss`,

		"filestream": `**Filestream Input:**
- Use prospector configurations for efficient file discovery
- Configure fingerprint-based file identity for rotated logs
- State tracking ensures no data loss across agent restarts`,

		"aws-s3": `**AWS S3 Input:**
- Use SQS notifications instead of polling for event-driven processing (more efficient)
- Configure visibility_timeout based on expected processing time
- Adjust max_number_of_messages for optimal batch size
- Use multiple agents consuming from the same SQS queue for horizontal scaling
- Configure Dead Letter Queue (DLQ) for failed message handling`,

		"kafka": `**Kafka Input:**
- Use consumer groups for horizontal scaling across multiple agents
- Ensure partition count allows for desired parallelism
- Consumer group offsets provide at-least-once delivery semantics`,

		"http_endpoint": `**HTTP Endpoint (Webhook) Input:**
- Deploy behind a load balancer for high availability
- Configure appropriate connection limits and timeouts
- Returns acknowledgment to sender, enabling retry on the sender side`,

		"aws-cloudwatch": `**AWS CloudWatch Input:**
- Adjust scan_frequency to balance freshness vs CloudWatch API costs
- Use log_group_name_prefix to limit scope and reduce API calls
- Be aware of CloudWatch API rate limits (10 requests/second by default)`,

		"cel": `**CEL (Common Expression Language) Input:**
- Optimize CEL expressions for performance to avoid CPU overhead
- Tune the evaluation interval if CEL is used for polling
- Be aware of potential rate limits if CEL expressions trigger external API calls`,

		"gcs": `**Google Cloud Storage Input:**
- Use Pub/Sub notifications for event-driven processing
- Configure appropriate poll_interval for polling mode
- Use bucket prefixes to limit scope`,

		"azure-blob-storage": `**Azure Blob Storage Input:**
- Use Event Grid notifications for efficient, event-driven processing
- Configure container name filters to limit scope
- Set appropriate poll_interval for polling mode`,

		"azure-eventhub": `**Azure Event Hub Input:**
- Scale horizontally by deploying multiple agents in a consumer group
- Monitor consumer lag to identify processing bottlenecks
- Adjust consumer_group and partitions for optimal throughput`,

		"gcp-pubsub": `**GCP Pub/Sub Input:**
- Use multiple agents with the same subscription for horizontal scaling
- Configure ack_deadline appropriately to prevent message redelivery
- Monitor subscription backlog and adjust max_messages for batch processing`,

		"sql": `**SQL/Database Input:**
- Optimize SQL queries for performance and use appropriate indexing
- Implement pagination for large query results to avoid memory issues
- Configure connection pooling to manage database load`,

		"netflow": `**Netflow/IPFIX Input:**
- UDP-based, so no guaranteed delivery; relies on network reliability
- Increase receive buffer size for the Netflow collector to prevent packet loss
- Distribute Netflow collection across multiple agents/collectors for high volume`,

		"winlog": `**Windows Event Log Input:**
- Filter events by event_id and channel to reduce volume
- Adjust batch_read_size for optimal performance
- Deploy agents locally on Windows hosts for efficient collection`,

		"journald": `**Journald Input:**
- Filter logs by unit or priority to reduce volume
- Monitor Journald's disk usage and rotation policies
- Adjust seek and cursor_file for persistent tracking`,

		"entity-analytics": `**Entity Analytics Input:**
- Optimize sync_interval to balance data freshness and API load
- Utilize incremental synchronization where supported to reduce data transfer
- Be aware of API rate limits from the entity source`,

		"o365audit": `**Office 365 Audit Input:**
- Filter by content_type to collect only necessary audit logs
- Be aware of Office 365 API throttling limits and adjust poll_interval
- Use multiple agents for different content types or tenants if needed`,

		"cloudfoundry": `**Cloud Foundry Input:**
- Filter logs by shard_id or app_filters to manage volume
- Deploy agents on Cloud Foundry VMs for efficient collection
- Monitor Loggregator throughput and agent resource usage`,

		"lumberjack": `**Lumberjack (Beats) Input:**
- Deploy a load balancer in front of multiple Elastic Agents to distribute connections
- Adjust queue.mem.events and queue.disk.events for buffering during backpressure
- Monitor network throughput and agent CPU/memory`,
	}
}

// BuildHeadStartContext is a convenience function that creates a context builder and builds the context
// This maintains backward compatibility with existing code
func BuildHeadStartContext(pkgCtx *validators.PackageContext, feedback []string) string {
	return NewContextBuilder(pkgCtx).WithFeedback(feedback).Build()
}

