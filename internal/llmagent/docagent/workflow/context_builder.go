// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package workflow provides workflow orchestration for documentation generation.
// This file contains the context builder used by the `update documentation` command
// to ensure consistent document generation across regular and evaluate modes.
package workflow

import (
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
)

// ContextBuilder builds rich context for the documentation generator.
// This is the single source of truth for generator context used by the
// `update documentation` command (both regular and --evaluate modes).
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

	// AGENT DEPLOYMENT - Standard agent deployment with input-specific network requirements
	sb.WriteString(b.buildAgentDeploymentGuidance())

	// VALIDATION - Standard verification steps with input-specific guidance
	sb.WriteString(b.buildValidationGuidance())

	// TROUBLESHOOTING - Input-specific troubleshooting guidance
	sb.WriteString(b.buildTroubleshootingGuidance())

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
	sb.WriteString("\n=== DATA STREAMS (document ALL of these in the Reference section) ===\n")
	sb.WriteString("CRITICAL: In the ## Reference section, each data stream MUST have:\n")
	sb.WriteString("  1. A ### heading with the data stream name\n")
	sb.WriteString("  2. A brief description of what data it collects\n")
	sb.WriteString("  3. {{event \"<name>\"}} template IF the data stream has an example event (marked with [has example])\n")
	sb.WriteString("  4. {{fields \"<name>\"}} template for field documentation\n\n")
	sb.WriteString("Data streams in this package:\n")
	for _, ds := range b.pkgCtx.DataStreams {
		sb.WriteString(fmt.Sprintf("- %s", ds.Name))
		if ds.Title != "" && ds.Title != ds.Name {
			sb.WriteString(fmt.Sprintf(" (%s)", ds.Title))
		}
		if ds.HasExampleEvent {
			sb.WriteString(" [has example]")
		}
		if ds.Description != "" {
			sb.WriteString(fmt.Sprintf(": %s", ds.Description))
		}
		sb.WriteString("\n")
		// Show the exact templates to use
		sb.WriteString(fmt.Sprintf("  → Use: {{fields \"%s\"}}", ds.Name))
		if ds.HasExampleEvent {
			sb.WriteString(fmt.Sprintf(" and {{event \"%s\"}}", ds.Name))
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
	sb.WriteString("6. Ensure heading hierarchy: # for title, ## for main sections, ### for subsections, #### for sub-subsections\n")
	sb.WriteString("7. In ## Reference section, use {{event \"<datastream_name>\"}} and {{fields \"<datastream_name>\"}} for EACH data stream (see DATA STREAMS section above for exact templates)\n")
	sb.WriteString("8. Address EVERY validation issue if any are listed above\n")
	sb.WriteString("9. For code blocks, always specify the language (e.g., ```bash, ```yaml)\n")
	sb.WriteString("10. Document ALL advanced settings with appropriate warnings (security, debug, SSL, etc.)\n")
	sb.WriteString("11. Use sentence case for headings (e.g., 'Vendor-side configuration' NOT 'Vendor-Side Configuration')\n")
	sb.WriteString("12. When showing example values like example.com, 10.0.0.1, or <placeholder>, add '(replace with your actual value)' or use format like `<your-hostname>`\n")
	sb.WriteString("13. Generate ONLY ONE H1 heading (the title) - all other headings should be H2 or lower\n")
	sb.WriteString("14. NEVER use # for code examples or configuration sections - use ### or #### instead\n")
	sb.WriteString("15. Heading levels must be sequential: H1 → H2 → H3 → H4 (never skip levels like H2 → H4)\n")
	sb.WriteString("16. In ## Troubleshooting, include BOTH vendor-specific issues AND input-specific troubleshooting tables from the guidance above\n")
	sb.WriteString("\n=== CONSISTENCY REQUIREMENTS ===\n")
	sb.WriteString("17. NEVER put bash comments (lines starting with #) outside code blocks - they will be parsed as H1 headings!\n")
	sb.WriteString("18. Use these EXACT subsection names in Troubleshooting:\n")
	sb.WriteString("    - '### Vendor-specific issues' (NOT 'Vendor-Specific Issues')\n")
	sb.WriteString("    - '### Vendor-specific issues' (NOT 'Vendor resources' or 'Vendor Resources')\n")
	sb.WriteString("    - '### [Input type] input troubleshooting' (e.g., 'TCP/Syslog input troubleshooting')\n")
	sb.WriteString("19. Use sentence case for ALL subsections (capitalize only first word): '### Log file input troubleshooting' NOT '### Log File Input Troubleshooting'\n")
	sb.WriteString("20. Under ## Reference, use:\n")
	sb.WriteString("    - '### Inputs used' (required)\n")
	sb.WriteString("    - '### API usage' (only for API-based integrations like httpjson)\n")
	sb.WriteString("    - '### Vendor documentation links' OR include links inline in relevant sections (NOT 'Vendor resources')\n")
	sb.WriteString("21. All code blocks MUST have language specified: ```bash, ```yaml, ```json - NEVER use bare ``` blocks\n")
	sb.WriteString("17. Include troubleshooting tables with columns: Symptom | Cause | Solution for each input type used\n")
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

// buildAgentDeploymentGuidance generates standard agent deployment instructions
// with input-type-specific network requirements derived from manifest.yml
func (b *ContextBuilder) buildAgentDeploymentGuidance() string {
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

	var sb strings.Builder
	sb.WriteString("\n=== AGENT DEPLOYMENT GUIDANCE (use this for ### Agent-based deployment section) ===\n")

	// Standard content - same for ALL integrations
	sb.WriteString(fmt.Sprintf(`
**Standard Agent Deployment Content (include this structure):**

The Elastic Agent is a unified agent that collects data from your systems and ships it to Elastic.
To deploy this integration:

1. **Install Elastic Agent** on a host that has network access to both your Elastic deployment and the data source.
   - See the [Elastic Agent installation guide](https://www.elastic.co/guide/en/fleet/current/install-fleet-managed-elastic-agent.html)

2. **Enroll the agent** in Fleet:
   - In Kibana, go to **Management** → **Fleet** → **Agents**
   - Click **Add agent** and follow the enrollment instructions

3. **Add the integration** to an agent policy:
   - Go to **Management** → **Integrations**
   - Search for "%s"
   - Click **Add %s** and configure the settings
   - Assign to an existing policy or create a new one
`, b.pkgCtx.Manifest.Title, b.pkgCtx.Manifest.Title))

	// Input-specific network requirements
	if len(inputTypes) > 0 {
		sb.WriteString("\n**Network Requirements (MUST include this table):**\n\n")
		sb.WriteString("| Direction | Protocol | Port | Purpose |\n")
		sb.WriteString("|-----------|----------|------|----------|\n")
		sb.WriteString("| Agent → Elastic | HTTPS | 443 | Data shipping to Elasticsearch |\n")

		networkReqs := getNetworkRequirements()
		for inputType := range inputTypes {
			if req, ok := networkReqs[inputType]; ok {
				sb.WriteString(req)
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// getNetworkRequirements returns network requirement table rows by input type
func getNetworkRequirements() map[string]string {
	return map[string]string{
		// Syslog inputs
		"tcp": "| Source → Agent | TCP | 514 (configurable) | Syslog reception |\n",
		"udp": "| Source → Agent | UDP | 514 (configurable) | Syslog reception |\n",

		// HTTP/API polling inputs
		"httpjson":         "| Agent → Source | HTTPS | 443 (varies) | API polling |\n",
		"cel":              "| Agent → Source | HTTPS | 443 (varies) | API access |\n",
		"http_endpoint":    "| Source → Agent | HTTP/HTTPS | 8080 (configurable) | Webhook reception |\n",
		"entity-analytics": "| Agent → Source | HTTPS | 443 | Entity provider API |\n",
		"salesforce":       "| Agent → Salesforce | HTTPS | 443 | Salesforce API |\n",
		"websocket":        "| Agent → Source | WSS | 443 (varies) | WebSocket connection |\n",

		// File-based inputs
		"logfile":    "| Agent (local) | — | — | File read access required |\n",
		"filestream": "| Agent (local) | — | — | File read access required |\n",
		"osquery":    "| Agent (local) | — | — | osquery socket access |\n",

		// AWS inputs
		"aws-s3":             "| Agent → AWS | HTTPS | 443 | S3/SQS API access |\n",
		"aws-cloudwatch":     "| Agent → AWS | HTTPS | 443 | CloudWatch API access |\n",
		"aws/metrics":        "| Agent → AWS | HTTPS | 443 | CloudWatch metrics API |\n",
		"awsfargate/metrics": "| Agent → AWS | HTTPS | 443 | ECS/Fargate metrics API |\n",

		// Azure inputs
		"azure-blob-storage": "| Agent → Azure | HTTPS | 443 | Blob Storage API access |\n",
		"azure-eventhub":     "| Agent → Azure | AMQP/HTTPS | 5671/443 | Event Hub connection |\n",
		"azure/metrics":      "| Agent → Azure | HTTPS | 443 | Azure Monitor API |\n",

		// GCP inputs
		"gcs":         "| Agent → GCP | HTTPS | 443 | GCS API access |\n",
		"gcp-pubsub":  "| Agent → GCP | HTTPS | 443 | Pub/Sub API access |\n",
		"gcp/metrics": "| Agent → GCP | HTTPS | 443 | Cloud Monitoring API |\n",

		// Message queue inputs
		"kafka":      "| Agent → Kafka | TCP | 9092 (varies) | Kafka broker connection |\n",
		"lumberjack": "| Beats → Agent | TCP | 5044 (configurable) | Beats protocol reception |\n",

		// Metrics inputs
		"prometheus/metrics": "| Agent → Source | HTTP | 9090 (varies) | Prometheus metrics scrape |\n",
		"kubernetes/metrics": "| Agent → K8s API | HTTPS | 443/6443 | Kubernetes API server |\n",
		"http/metrics":       "| Agent → Source | HTTP/HTTPS | varies | HTTP metrics endpoint |\n",
		"jolokia/metrics":    "| Agent → Source | HTTP | 8778 (varies) | JMX via Jolokia |\n",
		"statsd/metrics":     "| Source → Agent | UDP | 8125 (configurable) | StatsD reception |\n",
		"docker/metrics":     "| Agent (local) | — | — | Docker socket access |\n",
		"containerd/metrics": "| Agent (local) | — | — | Containerd socket access |\n",
		"system/metrics":     "| Agent (local) | — | — | System metrics (local) |\n",
		"etcd/metrics":       "| Agent → etcd | HTTP/HTTPS | 2379 | etcd metrics endpoint |\n",
		"memcached/metrics":  "| Agent → Memcached | TCP | 11211 | Memcached stats |\n",
		"zookeeper/metrics":  "| Agent → ZooKeeper | TCP | 2181 | ZooKeeper stats |\n",

		// Network capture inputs
		"netflow": "| Source → Agent | UDP | 2055 (configurable) | Netflow/IPFIX reception |\n",
		"packet":  "| Agent (local) | — | — | Network interface capture (promiscuous mode) |\n",

		// Windows inputs
		"winlog": "| Agent (local) | — | — | Windows Event Log API |\n",
		"etw":    "| Agent (local) | — | — | Event Tracing for Windows |\n",

		// Linux inputs
		"journald":             "| Agent (local) | — | — | Journald socket access |\n",
		"audit/auditd":         "| Agent (local) | — | — | Linux audit framework |\n",
		"audit/system":         "| Agent (local) | — | — | System audit events |\n",
		"audit/file_integrity": "| Agent (local) | — | — | File integrity monitoring |\n",

		// Database inputs
		"sql": "| Agent → Database | TCP | varies | Database connection |\n",

		// Cloud platform inputs
		"cloudfoundry": "| Agent → CF | HTTPS | 443 | Loggregator API access |\n",
		"o365audit":    "| Agent → Microsoft | HTTPS | 443 | Office 365 Management API |\n",

		// Connector inputs
		"connectors-py": "| Agent → Source | HTTPS | 443 (varies) | Python connector API |\n",

		// Specialized inputs
		"apm": "| Source → Agent | HTTP/HTTPS | 8200 (configurable) | APM data reception |\n",
	}
}

// buildValidationGuidance generates standard verification steps with input-specific guidance
func (b *ContextBuilder) buildValidationGuidance() string {
	if b.pkgCtx == nil || b.pkgCtx.Manifest == nil {
		return ""
	}

	packageName := b.pkgCtx.Manifest.Name
	packageTitle := b.pkgCtx.Manifest.Title

	// Extract unique input types from policy templates
	inputTypes := make(map[string]bool)
	for _, pt := range b.pkgCtx.Manifest.PolicyTemplates {
		for _, input := range pt.Inputs {
			if input.Type != "" {
				inputTypes[input.Type] = true
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("\n=== VALIDATION GUIDANCE (use this for ### Validation section) ===\n")
	sb.WriteString("Include these EXACT verification steps to ensure consistent documentation.\n\n")

	// Standard verification steps - same for ALL integrations
	sb.WriteString(`**Standard Verification Steps (include ALL of these):**

1. **Verify Elastic Agent status**
   - In Kibana, navigate to **Management** → **Fleet** → **Agents**
   - Confirm the agent status shows **Healthy** (green)
   - Click the agent name to verify the ` + packageTitle + ` integration is listed and shows no errors

2. **Check for incoming data in Discover**
   - Go to **Analytics** → **Discover**
   - Set the time range to **Last 15 minutes** or longer
   - Search for data from this integration using: ` + "`data_stream.dataset:" + packageName + "*`" + `
   - This will show ALL data streams for the integration (logs, metrics, etc.)
   - Verify documents are appearing with recent timestamps

3. **Verify specific data streams** (optional)
   - Data streams for this integration follow the naming pattern: ` + "`" + packageName + ".<datastream_name>`" + `
   - To check a specific data stream, filter by its full name (e.g., ` + "`data_stream.dataset:" + packageName + ".system`" + ` for system metrics)
   - Confirm events are being ingested for each enabled data stream

4. **Check dashboards**
   - Navigate to **Management** → **Integrations** → **` + packageTitle + `**
   - Click the **Assets** tab to see available dashboards
   - Open a dashboard to confirm visualizations are populated with data

`)

	// Input-specific verification guidance
	if len(inputTypes) > 0 {
		sb.WriteString("**Input-Specific Verification:**\n\n")
		verificationGuide := getInputVerificationGuidance()

		for inputType := range inputTypes {
			if guidance, ok := verificationGuide[inputType]; ok {
				sb.WriteString(guidance)
				sb.WriteString("\n")
			}
		}
	}

	// Note about data streams - don't list specific queries, just explain the pattern
	if len(b.pkgCtx.DataStreams) > 0 {
		sb.WriteString("**Available Data Streams for this integration:**\n")
		sb.WriteString("This integration includes the following data streams: ")
		dsNames := make([]string, 0, len(b.pkgCtx.DataStreams))
		for _, ds := range b.pkgCtx.DataStreams {
			dsNames = append(dsNames, ds.Name)
		}
		sb.WriteString(strings.Join(dsNames, ", "))
		sb.WriteString("\n\n")
		sb.WriteString("**IMPORTANT:** In the Validation section, tell users to search for `data_stream.dataset:" + packageName + "*` to see ALL data streams.\n")
		sb.WriteString("Do NOT list individual data stream queries - the wildcard pattern covers everything.\n\n")
	}

	return sb.String()
}

// getInputVerificationGuidance returns verification steps by input type
func getInputVerificationGuidance() map[string]string {
	return map[string]string{
		"httpjson": `**API Input Verification:**
- After adding the integration, wait for the configured polling interval to pass
- Check Discover for new documents with timestamps after the integration was added
- If no data appears after 2-3 polling intervals, check agent logs for API errors
- Verify API credentials are valid by checking for HTTP 401/403 errors in logs`,

		"cel": `**CEL Input Verification:**
- Wait for the configured interval to pass after adding the integration
- Check Discover for documents with recent timestamps
- If using pagination, verify all pages are being fetched by checking document counts
- Check agent logs for CEL expression errors or API failures`,

		"tcp": `**TCP/Syslog Verification:**
- Generate test traffic on the source system (e.g., trigger a log event)
- Check Discover within seconds for new syslog messages
- Verify the source IP in the events matches your sending device
- If no data, check firewall rules and verify the source is sending to the correct port`,

		"udp": `**UDP/Syslog Verification:**
- Generate test traffic on the source system immediately after setup
- Check Discover within seconds - UDP delivery is not guaranteed
- Verify source IP and port configuration
- **Note**: UDP does not confirm delivery; if data is missing, check network path`,

		"logfile": `**Log File Verification:**
- Add a test entry to the monitored log file or trigger an event that generates logs
- Check Discover within 10-30 seconds for the new entry
- Verify the file path in the integration matches the actual log location
- Check agent logs if the file is not being read`,

		"filestream": `**Filestream Verification:**
- Trigger an event that writes to the monitored log file
- Check Discover for new documents within 30 seconds
- Verify prospector is discovering files by checking agent logs
- Confirm file permissions allow the agent to read the log`,

		"aws-s3": `**S3 Input Verification:**
- Upload a test file to the monitored S3 bucket/prefix
- If using SQS, verify the message appears in the queue
- Check Discover within 1-2 minutes for processed documents
- Verify IAM permissions if no documents appear`,

		"aws-cloudwatch": `**CloudWatch Verification:**
- Generate activity that creates CloudWatch logs
- Wait for the scan_frequency interval to pass
- Check Discover for CloudWatch log events
- Verify the log group name/prefix matches your configuration`,

		"kafka": `**Kafka Input Verification:**
- Produce a test message to the configured Kafka topic
- Check Discover for the message within seconds
- Verify consumer group is active and consuming
- Check for consumer lag if messages are delayed`,

		"http_endpoint": `**Webhook Verification:**
- Send a test POST request to the webhook URL
- Check Discover immediately for the received event
- Verify the response (should return 200 OK)
- If using authentication, ensure the sender includes correct credentials`,

		"azure-eventhub": `**Event Hub Verification:**
- Send a test event to the Event Hub
- Check Discover within 1-2 minutes
- Verify consumer group checkpoints are updating
- Check for partition distribution if using multiple agents`,

		"gcp-pubsub": `**Pub/Sub Verification:**
- Publish a test message to the subscription's topic
- Check Discover within seconds
- Verify the subscription is receiving messages
- Check acknowledgment status in GCP Console`,

		"sql": `**SQL Input Verification:**
- Insert or update a test record in the monitored table
- Wait for the configured interval
- Check Discover for the new record
- Verify the tracking column is updating correctly`,

		"netflow": `**NetFlow/IPFIX Verification:**
- Generate network traffic on monitored interfaces
- Check Discover for flow records within 1-2 minutes
- Verify the collector is receiving UDP packets
- Check for template records if using IPFIX`,

		"winlog": `**Windows Event Log Verification:**
- Generate a test event (e.g., failed login, service start)
- Check Discover within seconds
- Filter by event.provider or event.code to find specific events
- Verify the event channel is correctly configured`,

		"journald": `**Journald Verification:**
- Generate a systemd log entry: ` + "`logger -t test 'Elastic Agent test'`" + `
- Check Discover within seconds
- Filter by systemd.unit if monitoring specific services
- Verify the agent has permissions to read journal`,

		"prometheus/metrics": `**Prometheus Metrics Verification:**
- Access the /metrics endpoint directly to verify it's responding
- Wait for the configured scrape interval
- Check Discover for metric documents
- Verify metric names match expected patterns`,

		"kubernetes/metrics": `**Kubernetes Metrics Verification:**
- Verify the agent pod is running in the cluster
- Check for kube-state-metrics or metrics-server availability
- Wait for the collection period
- Check Discover for kubernetes.* metrics`,

		"gcs": `**Google Cloud Storage Verification:**
- Upload a test file to the monitored GCS bucket/prefix
- If using Pub/Sub notifications, verify message delivery
- Check Discover within 1-2 minutes for processed documents
- Verify service account permissions if no documents appear`,

		"azure-blob-storage": `**Azure Blob Storage Verification:**
- Upload a test file to the monitored container
- If using Event Grid, verify event delivery
- Check Discover within 1-2 minutes
- Verify storage account access permissions`,

		"entity-analytics": `**Entity Analytics Verification:**
- Wait for the configured sync_interval to pass
- Check Discover for entity documents
- Verify API credentials if no data appears
- Check for incremental vs full sync behavior`,

		"o365audit": `**Office 365 Audit Verification:**
- Generate audit activity (e.g., file access, login)
- Note: O365 audit logs can have 24-48 hour delay
- Check Discover after waiting for content availability
- Verify Azure AD app permissions`,

		"cloudfoundry": `**Cloud Foundry Verification:**
- Generate application activity or logs
- Check Discover for CF events
- Verify shard_id configuration for multi-agent setups
- Check Loggregator connectivity`,

		"lumberjack": `**Lumberjack (Beats) Verification:**
- Configure a Beat to send to the agent endpoint
- Verify the Beat shows successful connection
- Check Discover for documents from the sending Beat
- Verify TLS certificates if using encryption`,

		"aws/metrics": `**AWS Metrics Verification:**
- Wait for the configured period to pass
- Check Discover for aws.* metric documents
- Verify IAM permissions for CloudWatch access
- Check for specific namespace metrics`,

		"azure/metrics": `**Azure Metrics Verification:**
- Wait for the configured interval
- Check Discover for azure.* metric documents
- Verify service principal permissions
- Check for specific resource metrics`,

		"gcp/metrics": `**GCP Metrics Verification:**
- Wait for the configured interval
- Check Discover for gcp.* metric documents
- Verify service account permissions
- Check Cloud Monitoring API quotas`,

		"connectors-py": `**Python Connector Verification:**
- Verify connector service is running
- Wait for the initial sync to complete
- Check Discover for synced documents
- Review connector logs for errors`,

		"packet": `**Network Packet Capture Verification:**
- Generate network traffic on the monitored interface
- Check Discover for packet/flow documents
- Verify the agent has CAP_NET_RAW capability
- Check interface name in configuration`,

		"websocket": `**WebSocket Verification:**
- Verify WebSocket connection is established (check agent logs)
- Wait for events on the WebSocket stream
- Check Discover for received messages
- Verify authentication if using secure WebSocket`,

		"etw": `**ETW (Event Tracing for Windows) Verification:**
- Generate activity that triggers the monitored provider
- Check Discover within seconds
- Verify the provider GUID is correct
- Ensure agent runs with admin privileges`,

		"salesforce": `**Salesforce Verification:**
- Generate activity in Salesforce (e.g., record update)
- Wait for the polling interval
- Check Discover for Salesforce events
- Verify OAuth token refresh is working`,

		"osquery": `**osquery Verification:**
- Verify osqueryd is running on the host
- Check that scheduled queries are executing
- Check Discover for osquery results
- Verify extensions socket path if using custom location`,

		"apm": `**APM Verification:**
- Generate application activity with APM agent instrumented
- Check Discover for APM traces, transactions, spans
- Verify APM server is receiving data
- Check APM agent configuration in your application`,

		"docker/metrics": `**Docker Metrics Verification:**
- Verify Docker daemon is running
- Run a container to generate activity
- Check Discover for docker.* metrics
- Verify agent has access to Docker socket`,

		"containerd/metrics": `**Containerd Metrics Verification:**
- Verify containerd is running
- Run a container to generate activity
- Check Discover for containerd.* metrics
- Verify agent has socket access`,

		"statsd/metrics": `**StatsD Metrics Verification:**
- Send a test metric: ` + "`echo 'test.metric:1|c' | nc -u -w1 127.0.0.1 8125`" + `
- Check Discover immediately for the metric
- Verify UDP port is not blocked
- Check metric format compatibility`,

		"audit/auditd": `**Linux Audit (auditd) Verification:**
- Generate auditable activity (e.g., file access, process execution)
- Check Discover for auditd events
- Verify agent has CAP_AUDIT_READ capability
- Check auditd rules are configured`,

		"audit/system": `**System Audit Verification:**
- Generate system events (login, process start)
- Check Discover for system audit events
- Verify agent permissions
- Check system audit configuration`,

		"audit/file_integrity": `**File Integrity Monitoring Verification:**
- Modify a monitored file
- Check Discover for FIM events
- Verify monitored paths are configured
- Check for initial baseline scan completion`,

		"jolokia/metrics": `**Jolokia (JMX) Metrics Verification:**
- Verify Jolokia agent is deployed on the Java application
- Access Jolokia endpoint directly to verify it responds
- Wait for the collection interval
- Check Discover for JMX metric documents`,

		"http/metrics": `**HTTP Metrics Verification:**
- Verify the metrics endpoint is accessible
- Wait for the configured period
- Check Discover for metric documents
- Verify endpoint authentication if required`,
	}
}

// buildTroubleshootingGuidance generates input-specific troubleshooting guidance
func (b *ContextBuilder) buildTroubleshootingGuidance() string {
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
	sb.WriteString("\n=== TROUBLESHOOTING GUIDANCE (MUST include in ## Troubleshooting section) ===\n")
	sb.WriteString("CRITICAL: ALL troubleshooting content must be SPECIFIC to this integration.\n")
	sb.WriteString("DO NOT include generic Elastic Agent debugging steps - those belong in common documentation.\n\n")
	sb.WriteString("Start with a link to common troubleshooting:\n")
	sb.WriteString("\"For help with Elastic ingest tools, check [Common problems](https://www.elastic.co/docs/troubleshoot/ingest/fleet/common-problems).\"\n\n")
	sb.WriteString("The Troubleshooting section must contain:\n")
	sb.WriteString("1. Vendor-specific issues (from service_info.md if any)\n")
	sb.WriteString("2. Input-specific troubleshooting tables for EACH input type this integration uses\n\n")

	// Add input-specific troubleshooting (no generic steps)
	troubleshootingKB := getTroubleshootingKnowledgeBase()
	for inputType := range inputTypes {
		if guidance, ok := troubleshootingKB[inputType]; ok {
			sb.WriteString(guidance)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}


// getTroubleshootingKnowledgeBase returns input-specific troubleshooting guidance
func getTroubleshootingKnowledgeBase() map[string]string {
	return map[string]string{
		"httpjson": `### API/HTTP JSON Input Troubleshooting

**No Data from API**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Invalid API credentials | Verify username/password or API key in integration settings. Test credentials using curl or Postman. |
| No documents indexed | API key lacks permissions | Ensure API key has read access to required endpoints. Check vendor documentation for required scopes/permissions. |
| No documents indexed | Incorrect API URL | Verify the base URL format (include https://, correct port). Test URL accessibility from agent host. |
| Intermittent data gaps | Rate limiting | Check agent logs for HTTP 429 errors. Increase polling interval in integration settings. |
| Connection timeouts | Network/firewall issues | Verify agent can reach API endpoint: ` + "`curl -v <api_url>`" + `. Check proxy settings if applicable. |

**Authentication Errors**
- Check agent logs for HTTP 401 (Unauthorized) or 403 (Forbidden) errors
- For OAuth: verify token endpoint URL, client ID/secret, and grant type
- For API keys: confirm key is active and not expired
- For basic auth: ensure username/password are URL-encoded if they contain special characters

**Debug API Requests**
Enable debug logging to see full request/response:
` + "```yaml" + `
# In agent policy, set log level to debug temporarily
# Check logs at: /var/lib/elastic-agent/data/elastic-agent-*/logs/
` + "```" + `
`,

		"cel": `### CEL (Common Expression Language) Input Troubleshooting

**No Data from CEL Input**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Invalid credentials | Verify API credentials (username/password, API key, or OAuth tokens). Test authentication independently. |
| No documents indexed | Incorrect API scope/permissions | Ensure credentials have access to required API endpoints. Check vendor docs for required OAuth scopes. |
| No documents indexed | CEL program error | Check agent logs for CEL evaluation errors. Validate CEL syntax using the CEL playground. |
| Partial data | Pagination issues | Review CEL program's cursor/pagination logic. Ensure state is properly maintained between polls. |
| Timeout errors | Large responses | Increase request timeout. Consider adding pagination to limit response size. |

**CEL Expression Debugging**
- Check agent logs for ` + "`cel`" + ` related errors
- Common issues: incorrect field paths, type mismatches, null pointer access
- Test CEL expressions with sample data before deployment

**API Credential Issues**
- OAuth tokens: verify refresh token flow is working
- API keys: check expiration, ensure correct scopes (read vs write)
- Service accounts: confirm account is active and has required roles
`,

		"tcp": `### TCP/Syslog Input Troubleshooting

**No Syslog Data Received**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Port not listening | Verify agent is listening: ` + "`netstat -tlnp | grep <port>`" + ` or ` + "`ss -tlnp | grep <port>`" + ` |
| No documents indexed | Firewall blocking | Check firewall rules allow inbound TCP on configured port. Test with ` + "`nc -zv <agent_ip> <port>`" + ` from source. |
| No documents indexed | Wrong destination IP | Verify source device is sending to correct agent IP address |
| Port conflict error | Port already in use | Choose a different port or stop the conflicting process. Check with ` + "`lsof -i :<port>`" + ` |
| Connection refused | Agent not started | Verify Elastic Agent service is running: ` + "`systemctl status elastic-agent`" + ` |

**Verify TCP Connectivity**
From the source device (or a test host):
` + "```bash" + `
# Test TCP connectivity
nc -zv <elastic_agent_ip> <syslog_port>

# Send a test message
echo "<14>Test syslog message" | nc <elastic_agent_ip> <syslog_port>
` + "```" + `

**Parsing Errors**
- If logs appear but fields are not parsed: verify log format matches expected format (RFC 3164, RFC 5424, CEF)
- Check for custom log formats that may need custom ingest pipeline
`,

		"udp": `### UDP/Syslog Input Troubleshooting

**No Syslog Data Received**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Port not listening | Verify agent is listening: ` + "`netstat -ulnp | grep <port>`" + ` or ` + "`ss -ulnp | grep <port>`" + ` |
| No documents indexed | Firewall blocking | Check firewall rules allow inbound UDP on configured port |
| No documents indexed | Packet loss | UDP doesn't guarantee delivery. Check for network congestion. Consider TCP instead. |
| Intermittent data loss | Buffer overflow | Increase UDP receive buffer: ` + "`sysctl -w net.core.rmem_max=26214400`" + ` |

**⚠️ UDP Data Loss Warning**
UDP does NOT guarantee delivery. Data loss occurs during:
- Network congestion
- Agent restarts
- Elasticsearch backpressure

**Strongly recommend TCP for production systems requiring data integrity.**

**Test UDP Reception**
` + "```bash" + `
# Send test UDP syslog message
echo "<14>Test syslog message" | nc -u <elastic_agent_ip> <syslog_port>
` + "```" + `
`,

		"logfile": `### Log File Input Troubleshooting

**No Log Data Collected**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Wrong file path | Verify path pattern matches actual log location. Use ` + "`ls -la <path>`" + ` to confirm. |
| No documents indexed | Permission denied | Ensure Elastic Agent user can read log files: ` + "`sudo -u elastic-agent cat <logfile>`" + ` |
| No documents indexed | File doesn't exist | Check log rotation - file may have been rotated. Verify logging is enabled on source. |
| Stale data only | File rotated | Configure ` + "`close_inactive`" + ` and ` + "`ignore_older`" + ` settings appropriately |
| Duplicate events | Registry corruption | Stop agent, remove registry file, restart agent (caution: may reprocess old logs) |

**Verify File Permissions**
` + "```bash" + `
# Check file exists and permissions
ls -la /path/to/logfile

# Test read access as elastic-agent user
sudo -u elastic-agent head /path/to/logfile

# Check parent directory permissions (need execute permission)
ls -la /path/to/
` + "```" + `

**Check File Position Registry**
The agent tracks its position in each file. If data seems stuck:
` + "```bash" + `
# View registry (location varies by OS)
cat /var/lib/elastic-agent/data/elastic-agent-*/run/filebeat/data/registry/filebeat/log.json
` + "```" + `
`,

		"filestream": `### Filestream Input Troubleshooting

**No Log Data Collected**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Path not matched | Verify path pattern. Use glob patterns like ` + "`/var/log/*.log`" + ` |
| No documents indexed | Permission denied | Ensure agent has read permissions on files and execute on directories |
| No documents indexed | Prospector not running | Check agent logs for prospector errors |
| Old data only | Fingerprint mismatch | File may have been truncated/recreated. Check prospector fingerprint settings |

**Verify File Discovery**
Check agent logs for messages about file discovery and harvesting.
`,

		"aws-s3": `### AWS S3 Input Troubleshooting

**No Data from S3**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Invalid credentials | Verify AWS access key/secret or IAM role has s3:GetObject permission |
| No documents indexed | Wrong bucket/prefix | Confirm bucket name and object prefix are correct |
| No documents indexed | SQS not configured | If using SQS notifications, verify queue URL and permissions |
| Access denied errors | Missing IAM permissions | Required: s3:GetObject, s3:ListBucket. For SQS: sqs:ReceiveMessage, sqs:DeleteMessage |

**Required IAM Permissions**
` + "```json" + `
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:ListBucket"],
      "Resource": ["arn:aws:s3:::bucket-name", "arn:aws:s3:::bucket-name/*"]
    }
  ]
}
` + "```" + `

**Test AWS Credentials**
` + "```bash" + `
# Test S3 access from agent host
aws s3 ls s3://bucket-name/prefix/ --region <region>
` + "```" + `
`,

		"aws-cloudwatch": `### AWS CloudWatch Input Troubleshooting

**No CloudWatch Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Invalid credentials | Verify AWS credentials have cloudwatch:GetLogEvents permission |
| No documents indexed | Wrong log group | Confirm log group name/prefix matches existing CloudWatch log groups |
| No documents indexed | Region mismatch | Ensure region setting matches where log groups exist |
| Rate limit errors | API throttling | Increase scan_frequency to reduce API calls |

**Required IAM Permissions**
` + "```json" + `
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "logs:DescribeLogGroups",
        "logs:DescribeLogStreams",
        "logs:GetLogEvents",
        "logs:FilterLogEvents"
      ],
      "Resource": "*"
    }
  ]
}
` + "```" + `
`,

		"kafka": `### Kafka Input Troubleshooting

**No Data from Kafka**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Connection failed | Verify broker addresses and port (default 9092) |
| No documents indexed | Authentication failed | Check SASL credentials or SSL certificates |
| No documents indexed | Consumer group issue | Verify consumer group has no conflicting consumers |
| Lag building up | Slow processing | Check agent resources, consider scaling horizontally |

**Test Kafka Connectivity**
` + "```bash" + `
# Test broker connectivity
nc -zv <broker_host> 9092

# List topics (requires kafka-topics CLI)
kafka-topics --bootstrap-server <broker>:9092 --list
` + "```" + `
`,

		"http_endpoint": `### HTTP Endpoint (Webhook) Input Troubleshooting

**No Webhook Data Received**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Port not listening | Verify agent is listening on configured port |
| No documents indexed | Firewall blocking | Ensure inbound HTTP/HTTPS traffic is allowed |
| No documents indexed | Wrong URL path | Verify sender is using correct webhook URL path |
| SSL errors | Certificate issues | Check SSL certificate validity and trust chain |

**Test Webhook Endpoint**
` + "```bash" + `
# Send test POST request
curl -X POST http://<agent_ip>:<port>/<path> \
  -H "Content-Type: application/json" \
  -d '{"test": "message"}'
` + "```" + `
`,

		"azure-eventhub": `### Azure Event Hub Input Troubleshooting

**No Event Hub Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Connection string invalid | Verify connection string includes EntityPath or separate event hub name |
| No documents indexed | Consumer group issue | Ensure consumer group exists and no other consumers are conflicting |
| No documents indexed | Storage account issue | Check storage account for checkpoint container access |
| Permission errors | Missing permissions | Verify shared access policy has Listen permission |

**Test Event Hub Connectivity**
Verify connection string format:
` + "`Endpoint=sb://<namespace>.servicebus.windows.net/;SharedAccessKeyName=<policy>;SharedAccessKey=<key>;EntityPath=<eventhub>`" + `
`,

		"gcp-pubsub": `### GCP Pub/Sub Input Troubleshooting

**No Pub/Sub Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Invalid credentials | Verify service account JSON key or workload identity |
| No documents indexed | Wrong subscription | Confirm subscription name and project ID are correct |
| No documents indexed | Permission denied | Service account needs roles/pubsub.subscriber role |
| Messages not acked | Timeout too short | Increase ack_deadline to allow for processing time |

**Required IAM Roles**
- roles/pubsub.subscriber (to receive messages)
- roles/pubsub.viewer (to list subscriptions)
`,

		"sql": `### SQL/Database Input Troubleshooting

**No Database Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Connection failed | Verify hostname, port, database name |
| No documents indexed | Authentication failed | Check username/password, SSL requirements |
| No documents indexed | Query returns empty | Test SQL query directly on database |
| No new data | Tracking column issue | Verify tracking column is indexed and updating |
`,

		"netflow": `### Netflow/IPFIX Input Troubleshooting

**No Netflow Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | UDP port not listening | Verify agent listening: ` + "`netstat -ulnp | grep 2055`" + ` |
| No documents indexed | Source not sending | Verify netflow export is enabled on network devices |
| No documents indexed | Firewall blocking | Ensure UDP 2055 (or custom port) is open |
| Template errors | Version mismatch | Check Netflow version (v5, v9, IPFIX) matches configuration |
`,

		"winlog": `### Windows Event Log Input Troubleshooting

**No Windows Events**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Wrong channel name | Verify event log channel name (e.g., Security, Application) |
| No documents indexed | Permission denied | Run agent as user with Event Log Readers group membership |
| No documents indexed | Channel disabled | Enable the event log channel in Windows Event Viewer |
| Missing events | Event ID filter | Check event_id filter isn't excluding desired events |
`,

		"journald": `### Journald Input Troubleshooting

**No Journald Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Permission denied | Ensure agent user is in 'systemd-journal' group |
| No documents indexed | Wrong unit filter | Verify systemd unit names in filter |
| No documents indexed | Journal not persistent | Check if journald is configured for persistent storage |
`,

		"prometheus/metrics": `### Prometheus Metrics Input Troubleshooting

**No Prometheus Metrics**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Wrong endpoint URL | Verify metrics endpoint path (usually /metrics) |
| No documents indexed | Target not exposing metrics | Check target is running and exposing Prometheus format metrics |
| No documents indexed | Authentication required | Configure bearer token or basic auth if endpoint requires it |
| Connection refused | Firewall/network | Verify agent can reach endpoint: ` + "`curl http://<host>:9090/metrics`" + ` |
`,

		"kubernetes/metrics": `### Kubernetes Metrics Input Troubleshooting

**No Kubernetes Metrics**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Missing RBAC permissions | Verify ServiceAccount has required ClusterRole bindings |
| No documents indexed | Wrong API server URL | Confirm kube_config or in_cluster settings |
| No documents indexed | Certificate issues | Check CA certificate configuration for API server |
| Authentication failed | Invalid credentials | Verify bearer token or kubeconfig is valid |
`,

		"aws/metrics": `### AWS Metrics Input Troubleshooting

**No AWS CloudWatch Metrics**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Invalid credentials | Verify AWS access key/secret or IAM role |
| No documents indexed | Missing permissions | Ensure cloudwatch:GetMetricData, cloudwatch:ListMetrics permissions |
| No documents indexed | Wrong region | Confirm region matches where resources exist |
| Incomplete metrics | Namespace filter | Check namespace and metric_name filters |
`,

		"azure/metrics": `### Azure Metrics Input Troubleshooting

**No Azure Monitor Metrics**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Invalid credentials | Verify client_id, client_secret, tenant_id |
| No documents indexed | Missing permissions | Ensure service principal has Monitoring Reader role |
| No documents indexed | Wrong subscription | Confirm subscription_id is correct |
| No metrics for resource | Resource not supported | Not all Azure resources expose metrics |
`,

		"gcp/metrics": `### GCP Metrics Input Troubleshooting

**No GCP Cloud Monitoring Metrics**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Invalid credentials | Verify service account JSON key |
| No documents indexed | Missing permissions | Service account needs roles/monitoring.viewer |
| No documents indexed | Wrong project | Confirm project_id is correct |
| API errors | Quota exceeded | Check Cloud Monitoring API quotas |
`,

		"connectors-py": `### Python Connector Input Troubleshooting

**No Data from Connector**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Invalid credentials | Verify API credentials for the data source |
| No documents indexed | Connector not running | Check connector service status and logs |
| No documents indexed | Network issues | Verify connectivity to both data source and Elasticsearch |
| Sync failures | Permission errors | Ensure credentials have read access to required data |
`,

		"packet": `### Network Packet Capture Input Troubleshooting

**No Packet Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Wrong interface | Verify network interface name in configuration |
| No documents indexed | Permission denied | Agent needs CAP_NET_RAW capability or root privileges |
| No documents indexed | No traffic | Ensure there's traffic on the monitored interface |
| Missing packets | BPF filter too restrictive | Review BPF filter syntax |
`,

		"websocket": `### WebSocket Input Troubleshooting

**No WebSocket Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Connection failed | Verify WebSocket URL (wss:// or ws://) |
| No documents indexed | Authentication failed | Check authentication credentials or tokens |
| Connection drops | Timeout/keepalive | Adjust ping_interval settings |
| SSL errors | Certificate issues | Configure SSL certificate authorities if using self-signed certs |
`,

		"etw": `### Event Tracing for Windows (ETW) Input Troubleshooting

**No ETW Events**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Wrong provider | Verify ETW provider GUID or name |
| No documents indexed | Permission denied | Agent must run with administrative privileges |
| No documents indexed | Provider not registered | Ensure ETW provider is registered on system |
| Session conflict | Trace session exists | Check for existing trace sessions using the provider |
`,

		"salesforce": `### Salesforce Input Troubleshooting

**No Salesforce Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Invalid credentials | Verify client_id, client_secret, username, password |
| No documents indexed | Security token missing | Append security token to password |
| No documents indexed | API access disabled | Enable API access for Salesforce user |
| Authentication failed | IP restrictions | Add agent IP to Salesforce trusted IP ranges |
`,

		"osquery": `### osquery Input Troubleshooting

**No osquery Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | osquery not running | Start osqueryd service: ` + "`systemctl start osqueryd`" + ` |
| No documents indexed | Wrong socket path | Verify osquery extensions socket path |
| No documents indexed | Permission denied | Agent needs access to osquery socket |
`,

		"apm": `### APM Input Troubleshooting

**No APM Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Port not listening | Verify APM server is listening on configured port (default 8200) |
| No documents indexed | Firewall blocking | Ensure inbound traffic allowed on APM port |
| No documents indexed | Agent not instrumented | Verify APM agents are configured to send to correct endpoint |
| Authentication failed | Secret token mismatch | Ensure APM agents use same secret_token as server |
`,

		"docker/metrics": `### Docker Metrics Input Troubleshooting

**No Docker Metrics**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Socket permission denied | Add agent user to docker group or configure socket permissions |
| No documents indexed | Wrong socket path | Default is /var/run/docker.sock - verify path |
| No documents indexed | Docker not running | Check Docker daemon status: ` + "`systemctl status docker`" + ` |
`,

		"containerd/metrics": `### Containerd Metrics Input Troubleshooting

**No Containerd Metrics**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Socket permission denied | Verify agent has access to containerd socket |
| No documents indexed | Wrong socket path | Check containerd socket location (usually /run/containerd/containerd.sock) |
`,

		"statsd/metrics": `### StatsD Metrics Input Troubleshooting

**No StatsD Metrics**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | Port not listening | Verify agent is listening on UDP port (default 8125) |
| No documents indexed | Wrong port in client | Ensure StatsD clients send to correct agent port |
| Metric name issues | Invalid metric format | Check StatsD metric format: ` + "`<metric_name>:<value>|<type>`" + ` |
`,

		"audit/auditd": `### Linux Audit (auditd) Input Troubleshooting

**No Audit Data**
| Symptom | Cause | Solution |
|---------|-------|----------|
| No documents indexed | auditd not running | Start auditd service: ` + "`systemctl start auditd`" + ` |
| No documents indexed | Permission denied | Agent needs root or CAP_AUDIT_READ capability |
| No documents indexed | No audit rules | Configure audit rules: ` + "`auditctl -l`" + ` to list rules |
`,
	}
}

// BuildHeadStartContext is a convenience function that creates a context builder and builds the context
// This maintains backward compatibility with existing code
func BuildHeadStartContext(pkgCtx *validators.PackageContext, feedback []string) string {
	return NewContextBuilder(pkgCtx).WithFeedback(feedback).Build()
}
