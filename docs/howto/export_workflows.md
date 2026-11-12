# HOWTO: Export workflows from Kibana

## Introduction

Integration packages can ship [Kibana Workflows](https://www.elastic.co/docs/explore-analyze/workflows) as assets, stored as YAML files in the `kibana/workflow` directory of the package. Fleet installs them as managed workflows when the package is installed.

Workflow assets are supported in integration packages of any `format_version`. Fleet installs them in Kibana 9.7.0 and later, and earlier versions ignore them, so set `conditions.kibana.version` in the package manifest accordingly (for example, `^9.7.0`).

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

Use the interactive prompt to select the workflows to export. The prompt lists the workflows authored by users. Workflows installed by Fleet are managed and read-only in Kibana, so they can't be exported; their source is already the file in the package.

Alternatively, pass a comma-separated list of workflow IDs with the `-d` flag:
```
elastic-package export workflows -d my-workflow-id,another-workflow-id
```

Each new workflow is stored in `kibana/workflow/{package name}-{workflow name}.yml`, where the workflow name is lowercased and any character other than `a-z` and `0-9` is replaced with `-`. For example, a workflow named `Alert Triage (v2)` in the `nginx` package is stored in `kibana/workflow/nginx-alert-triage-v2.yml`.

The exported YAML is kept as authored in Kibana, including comments, with these changes:
- `enabled` is set to `false`, as required for package assets. Users enable workflows after installing the package.

Existing workflow files with the same name are overwritten. Workflows made editable with `elastic-package edit workflows` are stored in the file they were installed from, even if their name changed (see [Edit a package workflow](#edit-a-package-workflow)).

## Check and build the package

Validate and build the package as usual:
```
elastic-package check
```

## Edit a package workflow

Workflows installed by Fleet are managed and read-only in Kibana. To change a workflow that is already part of the package, make an editable copy of it, edit the copy in Kibana, and export it back over the original file:

1. Install the package:
   ```
   elastic-package install
   ```
2. Make the workflow editable:
   ```
   elastic-package edit workflows
   ```
   Use the interactive prompt to select the package and its workflows, or pass a comma-separated list of the IDs of the installed workflows with the `-d` flag. For each workflow, the command creates an unmanaged copy whose ID is the name of the workflow file, lowercased and with `_` replaced by `-`. For example, `kibana/workflow/nginx-alert-triage.yml` gets the editable copy `nginx-alert-triage`. The command prints the Kibana URL of each copy.
3. Edit the copy in the Kibana UI.
4. Export it:
   ```
   elastic-package export workflows -d nginx-alert-triage
   ```
   The workflow is written to `kibana/workflow/nginx-alert-triage.yml`, overwriting the original file, even if it was renamed in Kibana.
5. Check the package and install it again.

Running `elastic-package edit workflows` again for a workflow that already has an editable copy leaves the copy unchanged, as it may contain edits that weren't exported yet.

Known limitations:
- The editable copy stays in Kibana as an unmanaged workflow next to the one installed by Fleet, also after reinstalling the package. Dashboards don't need this copy, because they can be switched to unmanaged.
- Kibana stores the installed workflow in its own format, so exporting a copy without changes may still show small formatting differences from the original file.
- Workflow IDs are unique across all Kibana spaces, and deleting a workflow in the UI keeps its ID taken. If the copy was deleted, `elastic-package edit workflows` fails with an "already exists" error until the workflow is deleted permanently, with `DELETE /api/workflows/workflow/{id}?force=true`.
