# HOWTO: Use a local or custom package registry for composable integrations

## Overview

Composable (integration) packages can declare required input packages in their `manifest.yml`
under `requires.input`. When you run `elastic-package build` or `elastic-package install`,
elastic-package resolves those dependencies by downloading them from the **package registry**.
By default it uses the production registry at `https://epr.elastic.co`.

This guide explains how to point elastic-package at a local or custom registry, which is
useful when the required input packages are still under development and not yet published to
the production registry.

For field-level build-time dependencies (ECS, `_dev/build/build.yml`), see
[HOWTO: Enable dependency management](./dependency_management.md).

## Prerequisites

- An integration package that declares `requires.input` in its `manifest.yml`, for example:

```yaml
requires:
  input:
    - package: sql_input
      version: "0.2.0"
```

- Optionally, a running local package registry that serves the required input packages.

## Option 1: Configure the registry URL globally

Edit `~/.elastic-package/config.yml` and set `package_registry.base_url` to your registry URL:

```yaml
package_registry:
  base_url: http://localhost:8080
```

This setting is read by `elastic-package build`, `elastic-package install`, and
`elastic-package status` when they need to contact the registry. It defaults to
`https://epr.elastic.co` when not set.

> **Note:** This setting does not change the package registry container that the Elastic Stack
> itself uses (served by `elastic-package stack`). To also redirect the stack, see
> [Option 2](#option-2-configure-the-registry-url-per-profile) below.

### Running a local package registry

To serve local packages, you can use the
[Elastic Package Registry](https://github.com/elastic/package-registry) with the
`--packages-path` flag pointing at a directory containing your built packages:

```shell
# Build your input package first
cd /path/to/sql_input
elastic-package build

# Start a local registry serving the build/ directory
docker run --rm -p 8080:8080 \
  -v /path/to/build/packages:/packages/package \
  docker.elastic.co/package-registry/distribution:latest \
  /package-registry serve --packages-path /packages
```

With a registry running at `http://localhost:8080`, set `package_registry.base_url` as shown
above and run `elastic-package build` from your integration package directory.

## Option 2: Configure the registry URL per profile

If you run `elastic-package stack` and want both the **build tools** and the **stack's Fleet**
to use the same custom registry, configure the active profile
(e.g. `~/.elastic-package/profiles/default/config.yml`):

```yaml
# URL for the stack's package registry container to proxy requests to
stack.epr.proxy_to: http://host.docker.internal:8080

# URL for the stack's package registry base endpoint
stack.epr.base_url: http://localhost:8080
```

Profile settings take precedence over the global `package_registry.base_url` for
stack-related behavior. The priority order is:

1. `stack.epr.proxy_to` (profile) — used as the proxy target for the local registry container
2. `stack.epr.base_url` (profile) — used as the registry base URL
3. `package_registry.base_url` (global `~/.elastic-package/config.yml`)
4. `https://epr.elastic.co` (production fallback)

For more details on profiles, see the
[Elastic Package profiles section of the README](../../README.md#elastic-package-profiles).

## Option 3: Use local source directories during tests (no registry required)

For **testing** only, you can bypass the registry entirely for specific required packages by
declaring **requires overrides** in `_dev/test/config.yml` with a `source` path pointing at a
local package directory:

```yaml
requires:
  - package: sql_input
    source: ../../sql_input
```

The `source` path can be absolute or relative to the package root. When the test runner builds
the package, it passes these overrides to the builder, which uses the local directory instead
of downloading from the registry.

You can also pin a version from the registry instead of using a local source:

```yaml
requires:
  - package: sql_input
    version: "0.1.0"
```

Overrides in `_dev/test/config.yml` apply to all test types. You can also scope them to a
specific test type:

```yaml
requires:
  - package: sql_input
    source: ../../sql_input

system:
  requires:
    - package: sql_input
      version: "0.2.0"
```

In this example, system tests use version `0.2.0` from the registry while all other test types
use the local source.

> **Note:** Requires overrides in `_dev/test/config.yml` only affect the **test runner's
> internal build step**. Running `elastic-package build` manually still uses the registry URL
> from the global config. Use Options 1 or 2 for the manual build workflow.

## Summary

| Goal | Configuration |
|------|--------------|
| Override registry for `build` / `install` | `package_registry.base_url` in `~/.elastic-package/config.yml` |
| Override registry for `stack` (Fleet) | `stack.epr.base_url` / `stack.epr.proxy_to` in the active profile `config.yml` |
| Use local package dir during tests | `requires[].source` in `_dev/test/config.yml` |
| Pin a specific version during tests | `requires[].version` in `_dev/test/config.yml` |
