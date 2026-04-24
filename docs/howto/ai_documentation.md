# AI-powered documentation generation

The `elastic-package update documentation` command uses a Large Language Model (LLM) to generate or update package documentation. It analyzes the package structure, data streams, and configuration files to produce a new documentation file based on the existing template and package context.

For a full list of flags, run:

```bash
elastic-package update documentation --help
```

## IMPORTANT PRIVACY NOTICE

When using AI-powered documentation generation, **file content from your local file system within the package directory may be sent to the configured LLM provider**. This includes manifest files, configuration files, field definitions, and other package content.

The generated documentation **must be reviewed for accuracy and correctness** before being finalized, as LLMs may occasionally produce incorrect or hallucinated information.

## Configuration

LLM provider settings can be supplied via environment variables or the elastic-package profile config file.


| Setting                | Environment variable           | Profile config key           |
| ---------------------- | ------------------------------ | ---------------------------- |
| Provider name          | `ELASTIC_PACKAGE_LLM_PROVIDER` | `llm.provider`               |
| Gemini API key         | `GOOGLE_API_KEY`               | `llm.gemini.api_key`         |
| Gemini model           | `GEMINI_MODEL`                 | `llm.gemini.model`           |
| Gemini thinking budget | `GEMINI_THINKING_BUDGET`       | `llm.gemini.thinking_budget` |


Currently only the Gemini provider is supported. Gemini is the default provider.

### Example profile config

```yaml
llm.provider: gemini
llm.gemini.api_key: "YOUR_API_KEY"
llm.gemini.model: "gemini-3-flash-preview"
llm.gemini.thinking_budget: "128"
```

## Service knowledge base (service_info.md)

To give the AI authoritative information about your integration, add a knowledge base file at `**docs/knowledge_base/service_info.md**` in your package. The generator treats this as the primary source and uses it to produce more accurate, vendor-specific documentation.

Using this file allows you to:

- Control the content of the generated documentation.
- Reduce the risk of hallucinations or incorrect information.
- Ensure consistent documentation when the file is updated in the future.

Guidelines:

- **Structure:** Use the structure template below. Section names are for organizing content for the LLM; they do not dictate the structure of the generated README.
- **Content:** Use specific, complete sentences. Include vendor documentation URLs and any known limitations or edge cases. Keep the file updated as the service or integration changes.
- **Effect:** The agent reads this file first, prioritizes it over other sources, and uses it to fill the corresponding parts of the generated docs. The format of `service_info.md` is not copied into the output.

### Optional template reference

The following is an optional reference for structuring `service_info.md`. Section titles are only to categorize information for the LLM; they do not control the format of the generated README.

```markdown
# Service Info

## Common use cases

/* Common use cases that this will facilitate */

## Data types collected

/* What types of data this integration can collect */

## Compatibility

/* Information on the vendor versions this integration is compatible with or has been tested against */

## Scaling and Performance

/* Vendor-specific information on what performance can be expected, how to set up scaling, etc. */

# Set Up Instructions

## Vendor prerequisites

/* Add any vendor specific prerequisites, e.g. "an API key with permission to access <X, Y, Z> is required" */

## Elastic prerequisites

/* If there are any Elastic specific prerequisites, add them here

    The stack version and agentless support is not needed, as this can be taken from the manifest */

## Vendor set up steps

/* List the specific steps that are needed in the vendor system to send data to Elastic.

  If multiple input types are supported, add instructions for each in a subsection */

## Kibana set up steps

/* List the specific steps that are needed in Kibana to add and configure the integration to begin ingesting data */

# Validation Steps

/* List the steps that are needed to validate the integration is working, after ingestion has started.

    This may include steps on the vendor system to trigger data flow, and steps on how to check the data is correct in Kibana dashboards or alerts. */

# Troubleshooting

/* Add lists of "*Issue* / *Solutions*" for troubleshooting knowledge base into the most appropriate section below */

## Common Configuration Issues

/* For generic problems such as "service failed to start" or "no data collected" */

## Ingestion Errors

/* For problems that involve "error.message" being set on ingested data */

## API Authentication Errors

/* For API authentication failures, credential errors, and similar */

# Documentation sites

/* List of URLs that contain info on the service (reference pages, set up help, API docs, etc.) */
```

