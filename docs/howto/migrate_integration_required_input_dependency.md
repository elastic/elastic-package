# HOWTO: Migrate an integration package to use required input dependencies

> **Reference implementation:** [elastic/integrations#19719](https://github.com/elastic/integrations/pull/19719) — `elastic_package_registry` migration from a legacy inline `prometheus/metrics` agent template to `prometheus_input` v1.0.1.

## Introduction

Integration packages historically duplicated agent collector configuration in local `agent/stream/*.ybs` templates. **Integrations with required input dependencies** delegate generic collector configuration to a reusable **input package** and keep the integration focused on service-specific defaults, field mappings, ingest pipelines, and dashboards.

At build time, `elastic-package build` fetches required input packages from the package registry, merges their agent templates and variable definitions into the integration, and rewrites `package:` references to the concrete input types Fleet expects. See [HOWTO: Enable dependency management](./dependency_management.md#integrations-with-required-packages-and-the-package-registry) for the general model.

This guide captures the practical migration steps and the non-obvious decisions surfaced during the first production migration ([`elastic_package_registry`](https://github.com/elastic/integrations/tree/master/packages/elastic_package_registry)).

## When to migrate

Migrate when:

- An input package already exists for the collector protocol your integration uses (for example [`prometheus_input`](https://github.com/elastic/integrations/tree/master/packages/prometheus_input) for Prometheus `/metrics` scraping).
- You want to remove duplicated stream configuration and inherit maintenance from the input package.
- Your target stack supports the required `format_version` (see [Prerequisites](#prerequisites)).

Do **not** assume a drop-in replacement: dataset naming, variable precedence, Fleet UI exposure, and policy test expectations all need explicit attention (see [Dataset management](#dataset-management) and [Variable overrides](#variable-overrides)).

## Prerequisites

### Package spec and stack support

- Bump `format_version` to **3.6.5** or later. `requires.input` is introduced in format version **3.6** (stack **9.4+**); use **3.6.5** as the minimum — later 3.6.x patch releases fixed spec validation issues found during early adoption. See [Guidelines for the format version](./format_version.md).
- Pin the input package under `requires.input` in the integration `manifest.yml` with an **exact** semver version (constraints are not supported for input dependencies):

  ```yaml
  requires:
    input:
      - package: prometheus_input
        version: "1.0.1"
  ```

- Update `conditions.kibana.version` if the migration requires a newer stack (for example `^9.4.4`). Other integrations typically record this as an `enhancement` changelog entry rather than a `breaking-change`, even when older stack versions are dropped.
- After the input package is published, use `elastic-package requires update` to refresh `requires.input` pins to the latest compatible registry version (see [Updating `requires` pins](./dependency_management.md#updating-requires-pins-from-the-package-registry)).

### Local development and tests

While the input package is unpublished or under active development, point test runners at a local copy via `_dev/test/config.yml`:

```yaml
policy:
  requires:
    - source: "../prometheus_input"
system:
  requires:
    - source: "../prometheus_input"
```

`source` overrides affect `elastic-package test` only. `elastic-package build` still fetches from the configured package registry unless you use a [local registry](./local_package_registry.md).

## Migration steps

The following checklist follows the order used in [elastic/integrations#19719](https://github.com/elastic/integrations/pull/19719).

### 1. Declare the input dependency in the package manifest

In `manifest.yml`:

1. Set `format_version: 3.6.5`.
2. Add `requires.input` with an exact input package version.
3. Update `policy_templates` to reference the input package instead of a raw agent input type:

   ```yaml
   policy_templates:
     - name: elastic_package_registry
       title: Elastic Package Registry metrics
       description: Collect metrics from Elastic Package Registry instances
       inputs:
         - package: prometheus_input
           title: Elastic Package Registry Metrics
           description: Collect Elastic Package Registry metrics via Prometheus exporters
   ```

### 2. Switch the data stream to `streams[].package`

In `data_stream/<name>/manifest.yml`, replace a legacy `input: prometheus/metrics` (or equivalent) stream definition with:

```yaml
streams:
  - package: prometheus_input
    enabled: true
    title: Elastic Package Registry metrics
    description: Collect Prometheus metrics from Elastic Package Registry
    template_path: stream.yml.hbs
    vars:
      # integration-specific variable overrides — see sections below
```

Only declare variables the integration actually wants to expose or override. The builder merges remaining variables from the input package manifest. See [Variable overrides](#variable-overrides) for the full decision tree; the [worked example table](#worked-example-elastic_package_registry-final-variable-set) lists the final `elastic_package_registry` set.

Variables can be declared at **stream level** (`streams[].vars` in the data stream manifest) or **input level** (`policy_templates[].inputs[].vars` in the package manifest). Input-level declarations are **promoted** — they become input-scoped variables in the merged manifest rather than stream-scoped ones. Use stream-level vars for per-data-stream tuning; use input-level vars when the override applies to every data stream that references the input package in that policy template.

### 3. Reduce the local agent stream template

Delete collector configuration that now comes from the input package (`metricsets`, `use_types`, `period`, `hosts`, `metrics_filters`, `processors`, `ssl`, and so on). Keep only values that are genuinely integration-owned.

After migration, `stream.yml.hbs` for `elastic_package_registry` contains only:

```yaml
metrics_path: {{metrics_path}}
tags:
{{#each tags as |tag i|}}
    - {{tag}}
{{/each}}
```

Everything else is merged from the `prometheus_input` template at build time.

If the integration shares agent templates via **link files** (`*.link`), set `template_path` to the **materialized** filename (without the `.link` suffix). The builder resolves templates after linked content is copied into the build output. See [Linked files and `template_path`](./dependency_management.md#linked-files-link-and-template_path).

### 4. Preserve the integration dataset

Set `dataset:` on the data stream manifest so documents keep the integration's index name instead of inheriting the input package default — see [Dataset management](#dataset-management).

### 5. Reconcile variable defaults and overrides

Audit each input variable and decide whether to inherit, redeclare with a new default, or expose as integration-only — see [Variable overrides](#variable-overrides).

### 6. Update tests

| Test type | What to add or update |
| --- | --- |
| **Policy** | Default test (`test-default.yml` with `vars: ~`) plus at least one **overrides** test covering variables removed from the local template (`metrics_filters.include`, `processors`, `tags`, integration-specific paths, and so on). Generate expectations with `elastic-package test policy --generate`. For integrations with multiple data streams sharing the same resolved input type, policy expectations must list sibling streams as `enabled: false` — Fleet auto-enables them otherwise. |
| **System** | Declare `requires` in `_dev/test/config.yml`; extend hit assertions for metrics that only appear under real traffic. |
| **Pipeline** | Add regression cases for edge conditions discovered during migration (for example null/absent `process_start_time_seconds`). |
| **Static / asset** | Re-run `elastic-package check` and dashboard export if Lens panel versions changed. |

Example overrides policy test (`test-overrides.yml`):

```yaml
data_stream:
  vars:
    metrics_path: /metrics_other
    metrics_filters.include:
      - "^epr_http_requests_total"
    processors:
      - rename:
          tag: rename_epr_http_requests_total_to_package_registry_http_requests_total_d512301d
          field: epr_http_requests_total
          target_field: package_registry.http_requests_total
          ignore_missing: true
    tags:
      - "test"
```

### 7. Update documentation

- Regenerate package docs (`elastic-package build` / docs pipeline).
- Manually document the input dependency if `{{ inputDocs }}` does not render (see [Documentation gaps](#documentation-gaps)).
- Add changelog entries: migration itself (`enhancement`), stack constraint bump (`enhancement`), field-mapping fixes (`bugfix`).

### 8. Verify end-to-end

```bash
elastic-package build
elastic-package test
elastic-package test policy -v
elastic-package test system
```

Manually exercise dashboards against a live service instance when metrics only populate under traffic (search/package API requests, and so on).

## Dataset management

Dataset naming is the most common silent regression when adopting `streams[].package`.

### What goes wrong

The `prometheus_input` package defines a `data_stream.dataset` variable with default `prometheus`. When an integration adopts the input package **without** overriding the dataset, Fleet renders policies with `dataset: prometheus`. Documents are indexed under `metrics-prometheus-*` instead of the integration's historical dataset (for example `elastic_package_registry.metrics`).

This was caught by policy tests in [elastic/integrations#19719](https://github.com/elastic/integrations/pull/19719): the expected policy targeted `metrics-elastic_package_registry.metrics-ep`, but the rendered policy used `prometheus`.

### Recommended approach: `dataset` on the data stream manifest

Set the dataset at the **data stream manifest** level, not as a user-facing variable:

```yaml
# data_stream/metrics/manifest.yml
title: "Prometheus Metrics"
type: metrics
# Preserves the integration dataset instead of inheriting the input package default.
# If not defined, Fleet falls back to <package>.<data_stream>.
dataset: elastic_package_registry.metrics
streams:
  - package: prometheus_input
    ...
```

This matches the approach explored in the closed nginx PR for required input dependencies ([elastic/integrations#18994](https://github.com/elastic/integrations/pull/18994)) and avoids exposing `data_stream.dataset` in the Fleet UI for integrations where users should not override the dataset.

### Approaches discussed but not preferred

| Approach | Issue |
| --- | --- |
| `data_stream.dataset` stream var with integration default | Works for policy rendering, but exposes a dataset override in Fleet UI that integrations typically do not want users to change. The input template references the variable, so Fleet merges the input default unless the integration overrides it. |
| Rely on Fleet auto-naming (`<package>.<data_stream>`) | Only correct when the historical dataset already matches that convention. Breaks existing dashboards, ILM patterns, and index permissions for established packages. |
| Hardcode dataset only in the local template | Insufficient when the input package template also references `data_stream.dataset`; the merged policy uses the variable value. |

### Platform status

Several dataset-handling improvements shipped after the reference migration:

- [elastic/elastic-package#3719](https://github.com/elastic/elastic-package/pull/3719) (**merged**) — excludes the input package's `data_stream.dataset` var from bundled stream-level vars so it does not leak into composable integrations as a user-facing override.
- [elastic/kibana#275312](https://github.com/elastic/kibana/pull/275312) (**merged**, stack 9.4+) — Fleet injects `data_stream.dataset` into template variables for composable integrations from package metadata.

An alternative elastic-package approach — rewriting `{{data_stream.dataset}}` to `{{_meta.stream.data_stream.dataset}}` in bundled templates — was explored in [elastic/elastic-package#3713](https://github.com/elastic/elastic-package/pull/3713) but was not merged.

**Integration-side recommendation:** explicitly set `dataset:` on the data stream manifest when the index name must differ from the input package default or from `<package>.<data_stream>`. This remains the most reliable approach for policy tests and for packages whose historical dataset does not match either fallback.

### Policy test expectations

After fixing the dataset, policy expectations should show:

```yaml
streams:
  - data_stream:
      dataset: elastic_package_registry.metrics
    ...
output_permissions:
  default:
    uuid-for-permissions-on-related-indices:
      indices:
        - names:
            - metrics-elastic_package_registry.metrics-ep
```

## Variable overrides

When an integration imports an input package, variables are merged from the input manifest into the integration/data stream manifest. Fleet shows them in the UI. The **rendered agent policy** is produced by merging:

1. Input package agent template defaults,
2. Integration `stream.yml.hbs` (integration-owned template),
3. User-selected variable values.

Understanding **which layer wins** is critical.

### Three categories of variables

Use this decision tree (from review discussion on [elastic/integrations#19719](https://github.com/elastic/integrations/pull/19719)):

#### A. Integration-specific values not present in the input package

Example: `metrics_path: /metrics` for Elastic Package Registry (the `prometheus_input` package has no `metrics_path` variable; hosts default to `localhost:9090/metrics`).

**Recommended:** add a data stream variable with the integration default and `show_user: false` when users should not change it:

```yaml
- name: metrics_path
  type: text
  title: Metrics Path
  default: /metrics
  required: true
  show_user: false
```

Reference it from the slim local template (`metrics_path: {{metrics_path}}`).

#### B. Input package variables where the integration needs a different default

Example: `rate_counters` — `prometheus_input` defaults to `true`, but Elastic Package Registry historically used `false`.

**Recommended:** redeclare the variable on the data stream `streams[].vars` with the integration's preferred default:

```yaml
- name: rate_counters
  type: bool
  title: Rate Counters
  description: Preferred value is false for Elastic Package Registry metrics.
  default: false
  show_user: false
```

Integration defaults **override** input defaults during manifest merge. Users can still change the value in Fleet unless `show_user: false`.

Example: `metrics_filters.exclude` with integration-specific defaults:

```yaml
- name: metrics_filters.exclude
  type: text
  multi: true
  default:
    - "^go_*"
    - "^promhttp_*"
```

#### C. Input package variables where the input default is acceptable

Example: `use_types` — input default is `true`, matching the legacy integration behaviour.

**Recommended:** do **not** redeclare the variable and do **not** hardcode it in `stream.yml.hbs`. Remove it from the local template and inherit from the input package. During bundling, inherited input-package vars are merged with `show_user: false` by default, so they appear under advanced options in Fleet without an explicit redeclaration.

### Hardcoded values in `stream.yml.hbs` vs manifest variables

During early migration it is tempting to hardcode overrides directly in `stream.yml.hbs`:

```yaml
rate_counters: false   # input default is true
use_types: true
```

This **does** affect the rendered agent policy, but creates a **Fleet UI gap**:

- Variables from the input package still appear in the UI (for example `rate_counters` shown as `true` because that is the input default).
- User changes to those UI fields have **no effect** because the local template hardcodes different values.

Reviewers flagged this as an implementation gap that should be addressed platform-side ([elastic/integrations#19719 review](https://github.com/elastic/integrations/pull/19719#discussion_r3537390759)). Until Fleet can hide or mark non-effective variables, prefer **manifest variable overrides** (category B) over silent template hardcoding.

### Variable override options compared

| Strategy | Pros | Cons |
| --- | --- | --- |
| Hardcode in `stream.yml.hbs` | Quick; guaranteed policy value | UI shows stale/input defaults; user edits ignored |
| Redeclare on data stream `vars` with new default | UI matches effective default; overridable; explicit | Must keep in sync when input package changes defaults |
| Redeclare with `show_user: false` | Hides advanced integration tuning | Still merged into policy; input package updates may need re-validation |
| Remove variable from input (platform) | Cleanest UX | Not available today; large input-package design decision |

### Worked example: `elastic_package_registry` final variable set

| Variable | Strategy | Notes |
| --- | --- | --- |
| `period` | Redeclare (`30s`) | Matches legacy integration |
| `hosts` | Redeclare, `required: true`, `show_user: true` | User-facing endpoint |
| `metrics_filters.exclude` | Redeclare with integration defaults | Replaces legacy template filters |
| `metrics_filters.include` | Redeclare (`default: []`, `show_user: true`) | Validated by overrides policy test |
| `processors` | Redeclare (`show_user: false`) | Validated by overrides policy test |
| `tags` | Redeclare | Rendered in local template |
| `metrics_path` | Redeclare (`show_user: false`) | Not in input package; EPR-specific |
| `rate_counters` | Redeclare `false` (`show_user: false`) | Overrides input default `true` |
| `use_types` | Inherit from input | Removed from local template |
| `ssl` | Inherit from input | Input default is a commented YAML string; behaviour matches `null`; no integration override needed |
| `data_stream.dataset` | **Not** a variable — use `dataset:` field | See [Dataset management](#dataset-management) |

### Policy test: default vs overrides

Always keep:

1. **`test-default.yml`** — `vars: ~` — proves defaults render correctly against the input package merge.
2. **`test-overrides.yml`** — sets variables that were removed from the local template to prove they still flow into the agent policy via merged input configuration.

Generate baselines:

```bash
cd packages/<your_package>
elastic-package test policy --generate
```

Review carefully: a `rate_counters: false` expectation is correct only when the integration overrides the input default, not when it is hardcoded invisibly in the template.

## Ingest pipelines, fields, and dashboards

These are not unique to migrations that adopt required input dependencies but surfaced during the reference PR:

### Ingest pipelines

Re-test pipelines against real collector output after the input package change. Prometheus label/layout differences and null metric values can expose latent pipeline assumptions. Add pipeline tests for null **and** missing fields when a processor chain assumes presence (for example `process_start_time_seconds`).

### Field mappings

Prometheus collector output may not match legacy hand-written types (for example counters mapped as `long` vs `double`). Fix mappings and add a `bugfix` changelog entry. `long → double` on counters is generally low risk for existing data; integer counter values remain compatible with existing `long` mappings until index rollover.

### Dashboards

Re-export or migrate dashboards when the target stack changes Lens panel versions (for example `formBased` datasources, panel version `8.9.0` for stack `9.4.4`). Validate dashboards under realistic traffic, not only against idle services.

## Documentation gaps

### `{{ inputDocs }}` does not render for `streams[].package`

For integrations using `streams[].package`, generated READMEs may leave the **Inputs used** section empty even when `{{ inputDocs }}` is present in `_dev/build/docs/README.md`. Tracked in [elastic/elastic-package#3696](https://github.com/elastic/elastic-package/issues/3696).

**Workaround:** manually document the input dependency and link to the input package docs until the builder supports integrations with required input dependencies, as done in `elastic_package_registry`:

```markdown
This integration uses the [Prometheus input](https://www.elastic.co/docs/reference/integrations/prometheus_input) to collect metrics from the `/metrics` endpoint...
```

### Empty ILM section in generated docs

Packages without ILM policies may still get an empty **ILM Policies** section in built READMEs ([`internal/docs/ilm.go`](../../internal/docs/ilm.go) lacks the empty-check used for transforms). Cosmetic only; tracked separately from this migration.

## Known platform gaps and follow-up issues

| Gap | Impact | Tracking |
| --- | --- | --- |
| Variables visible in UI but ignored by template | Confusing Fleet UX; risk of misconfiguration | Discussed in [elastic/integrations#19719](https://github.com/elastic/integrations/pull/19719); needs Fleet/input-package design |
| No integration-level opt-out for input variables | Cannot hide irrelevant input vars | Future enhancement |
| `{{ inputDocs }}` missing for integrations with required input dependencies | Incomplete generated docs | [elastic/elastic-package#3696](https://github.com/elastic/elastic-package/issues/3696) |
| Dataset variable vs manifest `dataset` field | Wrong index naming / permissions — mitigated by setting `dataset:` on the data stream manifest; bundler excludes input `data_stream.dataset` var ([#3719](https://github.com/elastic/elastic-package/pull/3719)); Fleet injects metadata ([#275312](https://github.com/elastic/kibana/pull/275312)) | Closed alternative: [elastic/elastic-package#3713](https://github.com/elastic/elastic-package/pull/3713) |

## Verification checklist

Before opening the migration PR:

- [ ] `format_version` ≥ `3.6.5` and `requires.input` pinned to a published input version
- [ ] `dataset` explicitly set on the data stream manifest when it must differ from the input default
- [ ] Local `stream.yml.hbs` contains only integration-owned template fragments
- [ ] Variable overrides use data stream `vars` (not silent template hardcoding) unless documented as intentional
- [ ] `_dev/test/config.yml` declares `requires` for local input package during development
- [ ] Policy tests: default + overrides, expectations reviewed for dataset, overridden defaults, and sibling streams (`enabled: false` where required)
- [ ] System tests pass with realistic service traffic where needed
- [ ] Pipeline regression tests for edge cases found during migration
- [ ] Changelog entries: migration, stack constraint, field-mapping fixes
- [ ] Docs manually updated if `{{ inputDocs }}` is empty
- [ ] Dashboards validated on the target stack version

## Related documentation

- [HOWTO: Enable dependency management](./dependency_management.md) — `requires.input`, test `source` overrides, `elastic-package requires update`
- [HOWTO: Writing policy tests](./policy_testing.md)
- [HOWTO: Package types](./package_types.md)
- [Guidelines for the format version](./format_version.md)
- [Reference implementation PR](https://github.com/elastic/integrations/pull/19719)
- [Reference package](https://github.com/elastic/integrations/tree/master/packages/elastic_package_registry)
