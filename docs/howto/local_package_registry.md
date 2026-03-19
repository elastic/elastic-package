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
package registry that serves all packages found under the `build/packages/` directory. You can
use this registry to serve locally-built input packages without running any additional
infrastructure.

```shell
# 1. Build the required input package — this places the built package under build/packages/
cd /path/to/sql_input
elastic-package build

# 2. Start the Elastic Stack from anywhere inside the repository.
#    elastic-package resolves the repository root automatically and the bundled
#    registry picks up build/packages/ from there.
elastic-package stack up -v -d
```

Then configure `~/.elastic-package/config.yml` to use the stack's local registry for
`elastic-package build` and `elastic-package install`:

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

### Alternative: standalone package registry container

If you are not running `elastic-package stack`, you can start a standalone registry container.
Use a port other than `8080` to avoid conflicting with the stack's built-in registry:

```shell
# Build your input package first
cd /path/to/sql_input
elastic-package build

# Start a standalone registry on port 8081
docker run --rm -p 8081:8080 \
  -v /path/to/build/packages:/packages/package \
  docker.elastic.co/package-registry/distribution:latest \
  /package-registry serve --packages-path /packages
```

Then point `package_registry.base_url` at `http://localhost:8081` and run
`elastic-package build` from your integration package directory.

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

## Summary

| Goal | Configuration |
|------|--------------|
| Override registry for `build` / `install` | `package_registry.base_url` in `~/.elastic-package/config.yml` |
| Override registry for `stack` (Fleet) | `stack.epr.base_url` / `stack.epr.proxy_to` in the active profile `config.yml` |
