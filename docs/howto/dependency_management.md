# HOWTO: Enable dependency management

## Motivation

As the package universe keeps growing, there are more occurrences of fields reusing by different integrations, especially
ones basing on the [Elastic Common Schema](https://github.com/elastic/ecs) (ECS). Without dependency management in place
developers tended to copy over same field definitions (mostly ECS related) from one integration to another, leading to
an increase of repository size and accidentally introducing inconsistencies. As there was no single source of truth defining
which field definition was correct, maintenance and typo correction process was expensive.

The described situation brought us to a point in time when a simple dependency management was a requirement to maintain
all used fields, especially ones imported from external sources.

Elastic Packages support two kinds of build-time dependency:

- **Field dependencies** — import field definitions from external schemas (e.g. ECS) using
  `_dev/build/build.yml`. Resolved from Git references and cached locally.
- **Package dependencies** — composable (integration) packages can depend on input and content packages
  declared under `requires` in `manifest.yml`. **Input package** dependencies are resolved
  at build time by downloading from the package registry. **Content package** dependencies are
  resolved at runtime by Fleet.

Both are described in the sections below.

## Principles of operation

Currently Elastic Packages support build-time field dependencies that can be used as external
field sources. They use a flat dependency model represented with an additional build manifest,
stored in an optional YAML file - `_dev/build/build.yml`:

```yaml
dependencies:
  ecs:
    reference: git@<commit SHA or Git tag>
```

When the elastic-package builds the package, it uses the build manifest to construct a dependencies map with references.

## External fields

While the builder processes fields files and encounters references to external sources, for example:

```yaml
- name: event.category
  external: ecs
- name: event.created
  external: ecs
- name: user_agent.os.full
  external: ecs
```

... it will try to resolve them using the prepared dependencies map and replace with actual definitions (importing).
The tool will try to download and cache locally referenced schemas (e.g. `git@0b8b7d6121340e99a1eb463c91fd1bc7c9eb2e41` or `git@1.10`).
Cached files are stored in a dedicated directory - `~/.elastic-package/cache/fields/`. It's assumed that schema (versioned) files
do not change.

To verify if building process went well, you can open `build` directory and compare fields (e.g. `./build/packages/nginx/1.2.3/access/fields/ecs.yml`):

```yaml
- description: |-
    This is one of four ECS Categorization Fields, and indicates the second level in the ECS category hierarchy.
    `event.category` represents the "big buckets" of ECS categories. For example, filtering on `event.category:process` yields all events relating to process activity. This field is closely related to `event.type`, which is used as a subcategory.
    This field is an array. This will allow proper categorization of some events that fall in multiple categories.
  name: event.category
  type: keyword
- description: |-
    event.created contains the date/time when the event was first read by an agent, or by your pipeline.
    This field is distinct from @timestamp in that @timestamp typically contain the time extracted from the original event.
    In most situations, these two timestamps will be slightly different. The difference can be used to calculate the delay between your source generating an event, and the time when your agent first processed it. This can be used to monitor your agent's or pipeline's ability to keep up with your event source.
    In case the two timestamps are identical, @timestamp should be used.
  name: event.created
  type: date
- description: Operating system name, including the version or code name.
  name: user_agent.os.full
  type: keyword
```

Fields in output fields files are stored sorted in alphabetical order.

### ECS repository

This dependency type refers to the ECS repository and allows for importing fields (name, type, description) from the common schema.
The schema is imported from the generated artifact (`generated/beats/fields.ecs.yml`) and it depends on a Git tag or a commit SHA.

To import fields from ECS v1.9, prepare the following `build.yml` file:

```yaml
dependencies:
  ecs:
    reference: git@1.9
```

and use a following field definition:

```yaml
- name: event.category
  external: ecs
```

## Composable packages and the package registry

Composable (integration) packages can also depend on input or content packages by declaring them under
`requires` in `manifest.yml`. Depending on the package type, dependencies are resolved
differently: **input package** dependencies are fetched at build time; **content package**
dependencies are resolved at runtime by Fleet.

```yaml
requires:
  input:
    - package: sql_input
      version: "0.2.0"
```

This type of dependency is resolved at **build time** by downloading the required input package
from the **package registry**. During `elastic-package build`, elastic-package fetches those
packages and updates the built integration: it bundles agent templates (policy and data stream),
merges variable definitions from the input packages into the composable manifest, adds data
stream field definitions where configured, and rewrites `package:` references on inputs and
streams to the concrete input types Fleet needs. Fleet still merges policy-specific values at
policy creation time.

Unlike field-level dependencies (which are resolved from Git references and cached locally),
package dependencies are fetched from the configured package registry URL
(`stack.epr.base_url` in the active profile, or `package_registry.base_url` in
`~/.elastic-package/config.yml`, defaulting to `https://epr.elastic.co`).

For details on using a local or custom registry when the required input packages are still
under development, see [HOWTO: Use a local or custom package registry](./local_package_registry.md).

### Testing composable packages with source overrides

When running `elastic-package test` on a composable integration whose required input packages
are not yet published to the registry, you can point each test runner at a local copy of the
input package using the `requires` key in `_dev/test/config.yml`.

Each entry in the `requires` list uses one of two forms:

- **`source`** — a path to a local package directory or `.zip` file. Relative paths are
  resolved relative to the composable package root. The package name is read from the
  `manifest.yml` at that path.
- **`package` + `version`** — forces a specific version to be fetched from the registry
  (useful in CI where the package is already published and you want to pin a version).

`source` and `package`/`version` are mutually exclusive in the same entry.

The `requires` key is supported under any test runner block: `policy`, `system`, `asset`,
`pipeline`, and `static`. You may define it in multiple blocks; if the same package appears
in more than one block, the resolved absolute paths must be identical.

```yaml
# _dev/test/config.yml — composable integration package
policy:
  requires:
    - source: "../my_input_pkg"       # local directory, relative to this package root
system:
  requires:
    - source: "../my_input_pkg"       # same override reused for system tests
asset:
  requires:
    - package: my_input_pkg           # registry-based override for asset tests
      version: "0.2.0"
```

> **Note:** Source overrides only affect the test runners (`elastic-package test`).
> `elastic-package build` always fetches required input packages from the configured
> package registry. To use a local registry during builds, see
> [HOWTO: Use a local or custom package registry](./local_package_registry.md).

A working example lives at
`test/packages/composable/02_ci_composable_integration/_dev/test/config.yml` (uses
`source: "../01_ci_input_pkg"`).

### Linked files (`*.link`) and `template_path`

Some repositories share agent templates using **link files** (files ending in `.link` that
point at shared content). During `elastic-package build`, linked content is copied into the
build output under the **target** path (the link filename without the `.link` suffix).

Composable bundling (`requires.input`) runs **after** linked files are materialized in the
build directory. In `manifest.yml`, always set `template_path` / `template_paths` to those
**materialized** names (for example `owned.hbs`), **not** the stub name (`owned.hbs.link`).
Fleet and the builder resolve templates by the names declared in the manifest; the `.link`
file exists only in the source tree.

A test fixture that combines `requires.input` with a linked policy input template
lives under `internal/requiredinputs/testdata/with_linked_template_path/`. Automated
coverage is in `TestBundleInputPackageTemplates_PreservesLinkedTemplateTargetPath` in
`internal/requiredinputs/requiredinputs_test.go`.
