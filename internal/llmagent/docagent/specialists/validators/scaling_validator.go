// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"fmt"
	"strings"
)

const (
	scalingValidatorName        = "scaling_validator"
	scalingValidatorDescription = "Validates Performance and scaling section provides input-specific guidance"
)

const scalingValidatorInstruction = `You are a documentation validator specializing in Elastic Agent performance and scaling guidance.
Your task is to verify that the "Performance and scaling" section provides specific, actionable advice based on the inputs used by the integration.

## Input
The documentation content and a list of inputs used by this integration are provided.

## Knowledge Base: Scaling Guidance by Input Type

### TCP Input (syslog/tcp)
- **Fault Tolerance**: TCP provides guaranteed delivery with acknowledgments. Recommend TCP over UDP for production.
- **Scaling**:
  - Multiple TCP listeners can be configured on different ports
  - Use load balancer to distribute across multiple Elastic Agents
  - Consider connection pooling limits on source systems
- **Backpressure**: TCP handles backpressure naturally; when Elasticsearch is slow, connections back up

### UDP Input (syslog/udp)
- **Fault Tolerance**: UDP is fire-and-forget with NO delivery guarantee. Data loss occurs during:
  - Network congestion
  - Agent restarts
  - Elasticsearch backpressure
- **Scaling**:
  - Increase receive buffer size (SO_RCVBUF) for high-volume environments
  - Consider multiple agents with DNS round-robin
- **CRITICAL WARNING**: For production systems requiring data integrity, strongly recommend TCP or message queue

### HTTP JSON Input (httpjson)
- **Scaling**:
  - Adjust polling interval to balance freshness vs load
  - Use request rate limiting to avoid overwhelming source API
  - Consider API rate limits from the vendor
- **Fault Tolerance**: Built-in retry with configurable backoff
- **Performance**: Use pagination for large datasets; configure appropriate timeouts

### Logfile Input (log/filestream)
- **Scaling**:
  - Use glob patterns to monitor multiple files
  - Configure appropriate harvester limits for many files
  - Use close_inactive to release file handles
- **Fault Tolerance**: Tracks file position in registry; survives agent restarts

### S3 Input (aws-s3)
- **Scaling**:
  - Use SQS notifications for event-driven processing (more efficient than polling)
  - Configure visibility_timeout based on processing time
  - Adjust max_number_of_messages for batch size
- **Fault Tolerance**: SQS provides guaranteed delivery; failed messages go to DLQ

### Kafka Input
- **Scaling**:
  - Consumer groups for horizontal scaling
  - Partition assignment across multiple agents
- **Fault Tolerance**: Consumer offsets provide exactly-once semantics

### HTTP Endpoint Input (http_endpoint)
- **Scaling**:
  - Configure behind load balancer for high availability
  - Adjust connection limits and timeouts
- **Fault Tolerance**: Returns acknowledgment to sender

### Cloudwatch Input
- **Scaling**:
  - Adjust scan_frequency for balance between freshness and API costs
  - Use log group filters to reduce scope
- **Performance**: Be aware of CloudWatch API rate limits

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "scaling", "location": "Performance and scaling", "message": "Issue description", "suggestion": "How to fix"}]}

## IMPORTANT
Output ONLY the JSON object. No other text.`

// InputScalingInfo contains scaling guidance for a specific input type
type InputScalingInfo struct {
	InputType         string
	DisplayName       string
	FaultTolerance    string
	ScalingGuidance   []string
	CriticalWarnings  []string
	RecommendTCPOver  bool // For UDP inputs, recommend TCP alternative
	RequiredTopics    []string
	RecommendedTopics []string
}

// ScalingValidator validates Performance and scaling section content
type ScalingValidator struct {
	BaseStagedValidator
}

// NewScalingValidator creates a new scaling validator
func NewScalingValidator() *ScalingValidator {
	return &ScalingValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        scalingValidatorName,
			description: scalingValidatorDescription,
			stage:       StageCompleteness, // Part of completeness checking
			scope:       ScopeBoth,         // Scaling validation works on sections and full document
			instruction: scalingValidatorInstruction,
		},
	}
}

// SupportsStaticValidation returns true
func (v *ScalingValidator) SupportsStaticValidation() bool {
	return true
}

// StaticValidate performs static validation of scaling documentation
func (v *ScalingValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StageCompleteness,
		Valid: true,
		Score: 100,
	}

	if pkgCtx == nil || pkgCtx.Manifest == nil {
		return result, nil
	}

	// Extract inputs from manifest
	inputs := v.extractInputTypes(pkgCtx)
	if len(inputs) == 0 {
		return result, nil
	}

	// Get scaling info for each input
	scalingInfos := v.getScalingInfoForInputs(inputs)

	// Extract the Performance and scaling section
	scalingSection := v.extractScalingSection(content)

	// Check if scaling section exists
	if scalingSection == "" {
		result.Issues = append(result.Issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryCompleteness,
			Location:    "Performance and scaling",
			Message:     "Missing or empty Performance and scaling section",
			Suggestion:  "Add a Performance and scaling section with guidance for the inputs used",
			SourceCheck: "static",
		})
		result.Valid = false
		result.Suggestions = v.buildSuggestions(scalingInfos)
		return result, nil
	}

	// Validate content covers required topics for each input
	result.Issues = append(result.Issues, v.validateScalingContent(scalingSection, scalingInfos)...)

	// Add suggestions for generator
	if len(result.Issues) > 0 {
		result.Suggestions = v.buildSuggestions(scalingInfos)
	}

	// Determine validity
	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	return result, nil
}

// extractInputTypes gets all input types from the manifest
func (v *ScalingValidator) extractInputTypes(pkgCtx *PackageContext) []string {
	inputSet := make(map[string]bool)

	for _, pt := range pkgCtx.Manifest.PolicyTemplates {
		for _, input := range pt.Inputs {
			if input.Type != "" {
				inputSet[input.Type] = true
			}
		}
	}

	inputs := make([]string, 0, len(inputSet))
	for input := range inputSet {
		inputs = append(inputs, input)
	}

	return inputs
}

// getScalingInfoForInputs returns scaling guidance for the given input types
func (v *ScalingValidator) getScalingInfoForInputs(inputs []string) []InputScalingInfo {
	var infos []InputScalingInfo

	knowledgeBase := v.getInputKnowledgeBase()

	for _, input := range inputs {
		if info, ok := knowledgeBase[input]; ok {
			infos = append(infos, info)
		} else {
			// Unknown input - add generic guidance
			infos = append(infos, InputScalingInfo{
				InputType:   input,
				DisplayName: input,
				ScalingGuidance: []string{
					"Document any scaling considerations specific to this input",
				},
				RecommendedTopics: []string{"scaling", "performance"},
			})
		}
	}

	return infos
}

// getInputKnowledgeBase returns the scaling knowledge base for all known inputs
func (v *ScalingValidator) getInputKnowledgeBase() map[string]InputScalingInfo {
	return map[string]InputScalingInfo{
		"tcp": {
			InputType:      "tcp",
			DisplayName:    "TCP/Syslog",
			FaultTolerance: "TCP provides guaranteed delivery with acknowledgments, making it suitable for production environments where data integrity is critical.",
			ScalingGuidance: []string{
				"Configure multiple TCP listeners on different ports for high availability",
				"Use a load balancer to distribute connections across multiple Elastic Agents",
				"Monitor connection limits on both source systems and the agent",
				"TCP handles backpressure naturally - connections queue when Elasticsearch is slow",
			},
			RequiredTopics:    []string{"tcp"},
			RecommendedTopics: []string{"load balancer", "connection", "listener", "availability"},
		},
		"udp": {
			InputType:        "udp",
			DisplayName:      "UDP/Syslog",
			FaultTolerance:   "UDP provides NO delivery guarantee. Data loss WILL occur during network congestion, agent restarts, or Elasticsearch backpressure.",
			RecommendTCPOver: true,
			ScalingGuidance: []string{
				"Increase receive buffer size (SO_RCVBUF) for high-volume environments",
				"Consider multiple agents with DNS round-robin for redundancy",
				"Monitor for packet loss using system metrics",
			},
			CriticalWarnings: []string{
				"UDP does not guarantee message delivery - consider TCP for production systems requiring data integrity",
				"During agent restarts or Elasticsearch slowdowns, UDP messages are silently dropped",
			},
			RequiredTopics:    []string{"udp"},
			RecommendedTopics: []string{"buffer", "reliability", "packet", "drop"},
		},
		"httpjson": {
			InputType:      "httpjson",
			DisplayName:    "HTTP JSON/API Polling",
			FaultTolerance: "Built-in retry mechanism with configurable exponential backoff handles transient failures.",
			ScalingGuidance: []string{
				"Adjust polling interval to balance data freshness vs API load",
				"Configure request rate limiting to avoid overwhelming source APIs",
				"Be aware of vendor API rate limits and adjust accordingly",
				"Use pagination for large datasets to avoid timeouts",
				"Configure appropriate request timeouts for your environment",
			},
			RequiredTopics:    []string{"interval", "poll", "api"},
			RecommendedTopics: []string{"rate limit", "timeout", "pagination", "retry"},
		},
		"logfile": {
			InputType:      "logfile",
			DisplayName:    "Log File",
			FaultTolerance: "File position tracking in registry survives agent restarts, ensuring no data loss.",
			ScalingGuidance: []string{
				"Use glob patterns to monitor multiple log files efficiently",
				"Configure harvester_limit to control resource usage with many files",
				"Use close_inactive setting to release file handles for rotated logs",
				"Set appropriate ignore_older to skip processing of old log files",
			},
			RequiredTopics:    []string{"file", "log"},
			RecommendedTopics: []string{"rotate", "harvester", "glob"},
		},
		"filestream": {
			InputType:      "filestream",
			DisplayName:    "Filestream",
			FaultTolerance: "State tracking ensures no data loss across agent restarts.",
			ScalingGuidance: []string{
				"Use prospector configurations for efficient file discovery",
				"Configure fingerprint-based file identity for rotated logs",
				"Set appropriate close.on_state_change settings",
			},
			RequiredTopics:    []string{"file"},
			RecommendedTopics: []string{"prospector", "fingerprint"},
		},
		"aws-s3": {
			InputType:      "aws-s3",
			DisplayName:    "AWS S3",
			FaultTolerance: "When used with SQS notifications, provides guaranteed delivery with automatic retries. Failed messages go to Dead Letter Queue.",
			ScalingGuidance: []string{
				"Use SQS notifications instead of polling for more efficient, event-driven processing",
				"Configure visibility_timeout based on expected processing time",
				"Adjust max_number_of_messages for optimal batch size",
				"Use multiple agents consuming from the same SQS queue for horizontal scaling",
				"Configure Dead Letter Queue for failed message handling",
			},
			RequiredTopics:    []string{"s3", "sqs"},
			RecommendedTopics: []string{"visibility", "batch", "queue", "dlq"},
		},
		"kafka": {
			InputType:      "kafka",
			DisplayName:    "Kafka",
			FaultTolerance: "Consumer group offsets provide at-least-once delivery semantics.",
			ScalingGuidance: []string{
				"Use consumer groups for horizontal scaling across multiple agents",
				"Ensure partition count allows for desired parallelism",
				"Configure appropriate fetch.min.bytes and fetch.wait.max for throughput",
			},
			RequiredTopics:    []string{"kafka", "consumer"},
			RecommendedTopics: []string{"partition", "offset", "consumer group"},
		},
		"http_endpoint": {
			InputType:      "http_endpoint",
			DisplayName:    "HTTP Endpoint (Webhook)",
			FaultTolerance: "Returns acknowledgment to sender, enabling retry on the sender side.",
			ScalingGuidance: []string{
				"Deploy behind a load balancer for high availability",
				"Configure appropriate connection limits and timeouts",
				"Monitor response times to ensure senders don't timeout",
			},
			RequiredTopics:    []string{"http", "endpoint", "webhook"},
			RecommendedTopics: []string{"load balancer", "timeout", "connection"},
		},
		"aws-cloudwatch": {
			InputType:      "aws-cloudwatch",
			DisplayName:    "AWS CloudWatch",
			FaultTolerance: "CloudWatch provides durable log storage; integration polls for new data.",
			ScalingGuidance: []string{
				"Adjust scan_frequency to balance freshness vs CloudWatch API costs",
				"Use log_group_name_prefix to limit scope and reduce API calls",
				"Be aware of CloudWatch API rate limits (10 requests/second by default)",
				"Consider regional deployment to reduce cross-region data transfer",
			},
			RequiredTopics:    []string{"cloudwatch"},
			RecommendedTopics: []string{"scan", "frequency", "api", "rate limit"},
		},
		"gcs": {
			InputType:      "gcs",
			DisplayName:    "Google Cloud Storage",
			FaultTolerance: "Tracks processed objects; survives restarts.",
			ScalingGuidance: []string{
				"Use Pub/Sub notifications for event-driven processing",
				"Configure appropriate poll_interval for polling mode",
				"Use bucket prefixes to limit scope",
			},
			RequiredTopics:    []string{"gcs", "storage"},
			RecommendedTopics: []string{"pubsub", "poll"},
		},
		"azure-blob-storage": {
			InputType:      "azure-blob-storage",
			DisplayName:    "Azure Blob Storage",
			FaultTolerance: "State tracking prevents duplicate processing.",
			ScalingGuidance: []string{
				"Use Event Grid notifications for efficient, event-driven processing",
				"Configure container name filters to limit scope",
				"Set appropriate poll_interval for polling mode",
			},
			RequiredTopics:    []string{"azure", "blob"},
			RecommendedTopics: []string{"event grid", "poll"},
		},
		"cel": {
			InputType:      "cel",
			DisplayName:    "CEL (Common Expression Language)",
			FaultTolerance: "Built-in retry mechanism with configurable backoff.",
			ScalingGuidance: []string{
				"Adjust the interval setting to balance data freshness vs source system load",
				"Configure request rate limiting if the source API has rate limits",
				"Use pagination (if supported by the API) for large result sets",
				"Consider the complexity of CEL expressions - simpler expressions perform better",
				"Monitor memory usage for large response payloads",
			},
			RequiredTopics:    []string{"cel", "interval"},
			RecommendedTopics: []string{"rate limit", "pagination", "memory"},
		},
		"azure-eventhub": {
			InputType:      "azure-eventhub",
			DisplayName:    "Azure Event Hub",
			FaultTolerance: "Consumer groups track offsets; at-least-once delivery.",
			ScalingGuidance: []string{
				"Use consumer groups for horizontal scaling across multiple agents",
				"Ensure partition count allows for desired parallelism",
				"Configure appropriate storage account for checkpointing",
			},
			RequiredTopics:    []string{"eventhub", "azure"},
			RecommendedTopics: []string{"consumer", "partition", "checkpoint"},
		},
		"gcp-pubsub": {
			InputType:      "gcp-pubsub",
			DisplayName:    "GCP Pub/Sub",
			FaultTolerance: "Pub/Sub provides at-least-once delivery with acknowledgments.",
			ScalingGuidance: []string{
				"Use multiple subscriptions for horizontal scaling",
				"Configure appropriate ack_deadline based on processing time",
				"Monitor subscription backlog for capacity planning",
			},
			RequiredTopics:    []string{"pubsub", "gcp"},
			RecommendedTopics: []string{"subscription", "ack", "backlog"},
		},
		"sql": {
			InputType:      "sql",
			DisplayName:    "SQL/Database",
			FaultTolerance: "Tracks last processed record; survives restarts.",
			ScalingGuidance: []string{
				"Use appropriate sql_query pagination (LIMIT/OFFSET or cursor-based)",
				"Index the tracking column for efficient queries",
				"Configure connection pooling for high-volume scenarios",
			},
			RequiredTopics:    []string{"sql", "database"},
			RecommendedTopics: []string{"pagination", "index", "connection"},
		},
		"netflow": {
			InputType:        "netflow",
			DisplayName:      "Netflow/IPFIX",
			FaultTolerance:   "UDP-based; data loss possible during congestion or restarts.",
			RecommendTCPOver: false, // No TCP alternative for netflow
			ScalingGuidance: []string{
				"Increase receive buffer size for high-volume environments",
				"Consider multiple collectors behind a load balancer",
				"Monitor for packet loss",
			},
			RequiredTopics:    []string{"netflow"},
			RecommendedTopics: []string{"buffer", "collector"},
		},
		"winlog": {
			InputType:      "winlog",
			DisplayName:    "Windows Event Log",
			FaultTolerance: "Bookmark tracking ensures no data loss across restarts.",
			ScalingGuidance: []string{
				"Use specific event IDs and channels to limit scope",
				"Configure batch_read_size for optimal throughput",
				"Monitor agent memory usage for high-volume channels",
			},
			RequiredTopics:    []string{"windows", "event"},
			RecommendedTopics: []string{"channel", "batch", "event id"},
		},
		"journald": {
			InputType:      "journald",
			DisplayName:    "Journald",
			FaultTolerance: "Cursor tracking ensures no data loss.",
			ScalingGuidance: []string{
				"Filter by specific systemd units to limit scope",
				"Configure appropriate seek position for initial collection",
			},
			RequiredTopics:    []string{"journald", "systemd"},
			RecommendedTopics: []string{"unit", "cursor"},
		},
		"entity-analytics": {
			InputType:      "entity-analytics",
			DisplayName:    "Entity Analytics",
			FaultTolerance: "State tracking for incremental sync.",
			ScalingGuidance: []string{
				"Configure appropriate sync_interval based on data change frequency",
				"Use incremental sync when possible to reduce API calls",
			},
			RequiredTopics:    []string{"entity", "analytics"},
			RecommendedTopics: []string{"sync", "incremental"},
		},
		"o365audit": {
			InputType:      "o365audit",
			DisplayName:    "Office 365 Management Activity API",
			FaultTolerance: "Content blob tracking ensures no duplicate processing.",
			ScalingGuidance: []string{
				"Configure appropriate interval based on audit log volume",
				"Be aware of Office 365 API throttling limits",
				"Use content type filters to limit scope",
			},
			RequiredTopics:    []string{"o365", "office"},
			RecommendedTopics: []string{"throttle", "content type"},
		},
		"cloudfoundry": {
			InputType:      "cloudfoundry",
			DisplayName:    "Cloud Foundry",
			FaultTolerance: "Tracks last received event.",
			ScalingGuidance: []string{
				"Configure appropriate shard_id for multi-agent deployments",
				"Use app filters to limit scope",
			},
			RequiredTopics:    []string{"cloudfoundry"},
			RecommendedTopics: []string{"shard", "app"},
		},
		"lumberjack": {
			InputType:      "lumberjack",
			DisplayName:    "Lumberjack (Beats protocol)",
			FaultTolerance: "Beats protocol provides acknowledgments.",
			ScalingGuidance: []string{
				"Deploy behind a load balancer for high availability",
				"Configure appropriate connection limits",
				"Monitor queue depth on sending Beats",
			},
			RequiredTopics:    []string{"lumberjack", "beats"},
			RecommendedTopics: []string{"load balancer", "queue"},
		},
	}
}

// isInputMentioned checks if an input type is mentioned in the section content
// using flexible matching to handle variations in naming
func isInputMentioned(sectionLower, inputType, displayName string) bool {
	// Strategy 1: Direct input type match (e.g., "httpjson", "tcp", "udp")
	if strings.Contains(sectionLower, inputType) {
		return true
	}

	// Strategy 2: Display name match (case-insensitive)
	displayNameLower := strings.ToLower(displayName)
	if strings.Contains(sectionLower, displayNameLower) {
		return true
	}

	// Strategy 3: Normalize special characters and retry
	// Replace "/" with " " or "-" and check again
	normalizedDisplay := strings.ReplaceAll(displayNameLower, "/", " ")
	if strings.Contains(sectionLower, normalizedDisplay) {
		return true
	}
	normalizedDisplay = strings.ReplaceAll(displayNameLower, "/", "-")
	if strings.Contains(sectionLower, normalizedDisplay) {
		return true
	}

	// Strategy 4: Check for key words from display name
	// For "HTTP JSON/API Polling", check if "http json" and ("api" or "polling") are present
	words := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(displayNameLower, "/", " "), "-", " "))
	matchedWords := 0
	for _, word := range words {
		if len(word) > 2 && strings.Contains(sectionLower, word) {
			matchedWords++
		}
	}
	// If most words (>50%) are present, consider it a match
	if len(words) > 0 && matchedWords >= (len(words)+1)/2 {
		return true
	}

	// Strategy 5: Common variations
	variations := map[string][]string{
		"httpjson": {"http json", "http-json", "http/json", "api polling", "json api"},
		"tcp":      {"tcp syslog", "tcp/syslog", "syslog tcp"},
		"udp":      {"udp syslog", "udp/syslog", "syslog udp"},
		"logfile":  {"log file", "file input", "log input"},
	}
	if alts, ok := variations[inputType]; ok {
		for _, alt := range alts {
			if strings.Contains(sectionLower, alt) {
				return true
			}
		}
	}

	return false
}

// extractScalingSection extracts the Performance and scaling section content
func (v *ScalingValidator) extractScalingSection(content string) string {
	contentLower := strings.ToLower(content)

	// Find the section
	patterns := []string{
		"## performance and scaling",
		"## performance & scaling",
		"## scaling and performance",
		"## performance",
		"## scaling",
	}

	for _, pattern := range patterns {
		idx := strings.Index(contentLower, pattern)
		if idx == -1 {
			continue
		}

		// Find the next H2 section or end of document
		rest := content[idx:]
		nextH2 := strings.Index(rest[3:], "\n## ")
		if nextH2 == -1 {
			return rest
		}
		return rest[:nextH2+3]
	}

	return ""
}

// validateScalingContent checks if the scaling section covers required topics
func (v *ScalingValidator) validateScalingContent(section string, scalingInfos []InputScalingInfo) []ValidationIssue {
	var issues []ValidationIssue
	sectionLower := strings.ToLower(section)

	for _, info := range scalingInfos {
		// Check for critical warnings (especially UDP)
		if len(info.CriticalWarnings) > 0 && info.RecommendTCPOver {
			// Check if the section mentions reliability concerns for UDP
			// Accept various ways of expressing "data loss" or "unreliable"
			hasDataLossWarning := strings.Contains(sectionLower, "data loss") ||
				strings.Contains(sectionLower, "packet loss") ||
				strings.Contains(sectionLower, "packets may be dropped") ||
				strings.Contains(sectionLower, "dropped") ||
				strings.Contains(sectionLower, "no guarantee") ||
				strings.Contains(sectionLower, "does not guarantee") ||
				strings.Contains(sectionLower, "not guaranteed") ||
				strings.Contains(sectionLower, "unreliable")

			hasTCPRecommendation := strings.Contains(sectionLower, "tcp") &&
				(strings.Contains(sectionLower, "recommend") ||
					strings.Contains(sectionLower, "consider") ||
					strings.Contains(sectionLower, "prefer") ||
					strings.Contains(sectionLower, "instead"))

			if !hasDataLossWarning && !hasTCPRecommendation {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityCritical,
					Category:    CategoryCompleteness,
					Location:    "Performance and scaling",
					Message:     fmt.Sprintf("%s input: Missing warning about UDP data loss and TCP recommendation", info.DisplayName),
					Suggestion:  "Add warning that UDP does not guarantee delivery and recommend TCP for production systems requiring data integrity",
					SourceCheck: "static",
				})
			}
		}

		// Check for required topics - more lenient check
		// Only flag as major if the input type itself isn't mentioned
		inputMentioned := isInputMentioned(sectionLower, info.InputType, info.DisplayName)

		if !inputMentioned {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryCompleteness,
				Location:    "Performance and scaling",
				Message:     fmt.Sprintf("%s input: Section lacks specific scaling guidance for this input type", info.DisplayName),
				Suggestion:  fmt.Sprintf("Add scaling guidance for %s: %s", info.InputType, strings.Join(info.ScalingGuidance[:min(2, len(info.ScalingGuidance))], "; ")),
				SourceCheck: "static",
			})
		}

		// Check for recommended topics (minor if missing)
		missingRecommended := 0
		for _, topic := range info.RecommendedTopics {
			if !strings.Contains(sectionLower, topic) {
				missingRecommended++
			}
		}

		if missingRecommended == len(info.RecommendedTopics) && len(info.RecommendedTopics) > 0 {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryCompleteness,
				Location:    "Performance and scaling",
				Message:     fmt.Sprintf("%s input: Consider adding more specific scaling recommendations", info.DisplayName),
				Suggestion:  fmt.Sprintf("Consider mentioning: %s", strings.Join(info.RecommendedTopics, ", ")),
				SourceCheck: "static",
			})
		}
	}

	// Check for generic/vague content
	vagueIndicators := []string{
		"consider increasing",
		"may need to",
		"might want to",
		"as needed",
	}

	specificIndicators := []string{
		"increase", "decrease", "configure", "set", "adjust",
		"buffer", "timeout", "interval", "limit", "connection",
	}

	vagueCount := 0
	specificCount := 0

	for _, indicator := range vagueIndicators {
		if strings.Contains(sectionLower, indicator) {
			vagueCount++
		}
	}

	for _, indicator := range specificIndicators {
		if strings.Contains(sectionLower, indicator) {
			specificCount++
		}
	}

	if vagueCount > specificCount && len(section) > 100 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryCompleteness,
			Location:    "Performance and scaling",
			Message:     "Section contains vague guidance without specific recommendations",
			Suggestion:  "Provide specific configuration parameters and values where possible",
			SourceCheck: "static",
		})
	}

	return issues
}

// buildSuggestions creates actionable suggestions for the generator
func (v *ScalingValidator) buildSuggestions(scalingInfos []InputScalingInfo) []string {
	var suggestions []string

	suggestions = append(suggestions, "## Performance and Scaling - Required Content")
	suggestions = append(suggestions, "")

	for _, info := range scalingInfos {
		suggestions = append(suggestions, fmt.Sprintf("### %s Input (%s)", info.DisplayName, info.InputType))

		if info.FaultTolerance != "" {
			suggestions = append(suggestions, fmt.Sprintf("**Fault Tolerance**: %s", info.FaultTolerance))
		}

		if len(info.CriticalWarnings) > 0 {
			suggestions = append(suggestions, "**⚠️ CRITICAL WARNINGS**:")
			for _, warning := range info.CriticalWarnings {
				suggestions = append(suggestions, fmt.Sprintf("- %s", warning))
			}
		}

		if len(info.ScalingGuidance) > 0 {
			suggestions = append(suggestions, "**Scaling Guidance**:")
			for _, guidance := range info.ScalingGuidance {
				suggestions = append(suggestions, fmt.Sprintf("- %s", guidance))
			}
		}

		suggestions = append(suggestions, "")
	}

	return suggestions
}
