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

## Option 1: Use the built-in stack registry (recommended)

`elastic-package stack up` (with the default compose provider) automatically starts a local
package registry container. The container runs in **proxy mode**: it serves packages found in
the repository's `build/packages/` directory and proxies all other package requests to the
production registry at `https://epr.elastic.co` (or to a custom upstream if configured).

`elastic-package` discovers `build/packages/` by walking up from the current working
directory to the repository root, so you can run `elastic-package stack up` from anywhere
inside the repository.

```shell
# 1. Build the required input package — this places the built package under build/packages/
#    at the repository root.
cd /path/to/sql_input
elastic-package build

# 2. Start the Elastic Stack from anywhere inside the repository.
#    The bundled registry picks up build/packages/ from the repository root.
elastic-package stack up -v -d
```

Then configure `~/.elastic-package/config.yml` to use the stack's local registry for
`elastic-package build`, `elastic-package test`, `elastic-package benchmark`, and
`elastic-package status`:

```yaml
package_registry:
  base_url: http://localhost:8080
```

This setting defaults to `https://epr.elastic.co` when not set.

> **Note:** This setting does not change the package registry container that the Elastic Stack
> itself uses (served by `elastic-package stack`). To also redirect the stack's proxy target,
> see [Option 2](#option-2-configure-the-registry-url-per-profile) below.

### Alternative: standalone package registry container

If you are not running `elastic-package stack`, you can start a standalone registry container.
Use a port other than `8080` to avoid conflicting with the stack's built-in registry:

```shell
# Build your input package first
cd /path/to/sql_input
elastic-package build

# Start a standalone registry on port 8081, mounting the build/packages/ directory
# at the repository root (run from anywhere inside the repo, or adjust the path).
docker run --rm -p 8081:8080 \
  -v "$(git -C /path/to/repo rev-parse --show-toplevel)/build/packages":/packages/package-registry \
  docker.elastic.co/package-registry/package-registry:v1.37.0
```

> **Note:** The mounted directory must contain at least one valid package (a `.zip` file or an
> extracted package directory). If the directory is empty, the registry exits immediately with
> `No local packages found.`
>
> **Note:** The registry image tag above matches `PackageRegistryBaseImage` in
> [`internal/stack/versions.go`](../../internal/stack/versions.go); that constant is what
> `elastic-package stack` uses and is updated by automation, while this document is not —
> check there when upgrading.

Then point `package_registry.base_url` at `http://localhost:8081` and run
`elastic-package build` from your integration package directory.

## Option 2: Configure the registry URL per profile

Use this option when you want both the **build tools** and the **stack's Fleet** to use the
same custom or standalone registry — for example, a registry serving packages not yet
published to production.

Assume your custom registry is running on the host at port `8082`. Configure the active
profile (e.g. `~/.elastic-package/profiles/default/config.yml`):

```yaml
# The stack's package registry container will proxy non-local requests to this URL.
# Use host.docker.internal so the container can reach the host.
stack.epr.proxy_to: http://host.docker.internal:8082

# elastic-package install (and stack commands) will use this URL to contact the registry.
stack.epr.base_url: http://localhost:8082
```

To also cover `elastic-package test`, `elastic-package benchmark`, and `elastic-package status`
(which do not read profile settings), add the global setting:

```yaml
# ~/.elastic-package/config.yml
package_registry:
  base_url: http://localhost:8082
```

### URL resolution reference

**For `elastic-package build`** (profile, then global config):

| Priority | Setting |
| -------- | ------- |
| 1 | `stack.epr.base_url` in the active profile `config.yml` |
| 2 | `package_registry.base_url` in `~/.elastic-package/config.yml` |
| 3 | `https://epr.elastic.co` (production fallback) |

**For `elastic-package test`, `benchmark`, `status`** (global config only):

| Priority | Setting |
| -------- | ------- |
| 1 | `package_registry.base_url` in `~/.elastic-package/config.yml` |
| 2 | `https://epr.elastic.co` (production fallback) |

**For `elastic-package install` and stack commands** (profile takes precedence):

| Priority | Setting |
| -------- | ------- |
| 1 | `stack.epr.base_url` in the active profile `config.yml` |
| 2 | `package_registry.base_url` in `~/.elastic-package/config.yml` |
| 3 | `https://epr.elastic.co` (production fallback) |

**For the stack registry's proxy target** (`EPR_PROXY_TO` inside the container):

| Priority | Setting |
| -------- | ------- |
| 1 | `stack.epr.proxy_to` in the active profile `config.yml` |
| 2 | `stack.epr.base_url` in the active profile `config.yml` |
| 3 | `package_registry.base_url` in `~/.elastic-package/config.yml` |
| 4 | `https://epr.elastic.co` (production fallback) |

For more details on profiles, see the
[Elastic Package profiles section of the README](../../README.md#elastic-package-profiles).

## Summary

| Goal | Configuration |
| ---- | ------------- |
| Override registry for `build` | `stack.epr.base_url` in the active profile `config.yml` (or `package_registry.base_url` in `~/.elastic-package/config.yml`) |
| Override registry for `test` / `benchmark` / `status` | `package_registry.base_url` in `~/.elastic-package/config.yml` |
| Override registry for `install` and stack commands | `stack.epr.base_url` in the active profile `config.yml` |
| Override proxy target for the stack's registry container | `stack.epr.proxy_to` in the active profile `config.yml` |
