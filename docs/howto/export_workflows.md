# HOWTO: Export workflows from Kibana

## Introduction

Integration packages can ship [Kibana Workflows](https://www.elastic.co/docs/explore-analyze/workflows) as assets, stored as YAML files in the `kibana/workflow` directory of the package. Fleet installs them as managed workflows when the package is installed.

Workflow assets require an integration package with `format_version: 3.8.0` or later, and Kibana 9.7.0 or later set in the package manifest with `conditions.kibana.version`. `elastic-package check` reports an error if the package doesn't meet these requirements.

## Author a workflow in Kibana

Start a stack and create the workflow in the Kibana UI. Workflows can be exported from Kibana 9.5 and later; to install workflow assets from packages, Fleet needs the workflow assets feature, which can be enabled in the stack started by elastic-package with:
```
elastic-package stack up -d -U stack.fleet_workflow_assets_enabled=true
```

## Export the workflow

From the package directory, run:
```
elastic-package export workflows
```

Use the interactive prompt to select the workflows to export. By default, the prompt only lists the workflows authored by users. Use `--managed` to also list the workflows that Fleet installed from the current package.

Alternatively, pass a comma-separated list of workflow IDs with the `-d` flag:
```
elastic-package export workflows -d my-workflow-id,another-workflow-id
```

Each workflow is stored in `kibana/workflow/{package name}-{workflow name}.yml`, where the workflow name is lowercased and any character other than `a-z` and `0-9` is replaced with `-`. For example, a workflow named `Alert Triage (v2)` in the `nginx` package is stored in `kibana/workflow/nginx-alert-triage-v2.yml`. Workflows that Fleet installed from the package are written back to the file they were installed from.

The exported YAML is kept as authored in Kibana, including comments, with these changes:
- `enabled` is set to `false`, as required for package assets. Users enable workflows after installing the package.

Known issue: for workflows that Fleet installed from the package, Kibana returns YAML regenerated from the stored definition, not the file shipped in the package. Exporting them with `--managed` drops the comments and the original formatting of the package file, for example by folding multi-line queries. This will be addressed when editing package workflows in Kibana is supported.

Existing workflow files with the same name are overwritten.

## Check and build the package

Validate and build the package as usual:
```
elastic-package check
```
