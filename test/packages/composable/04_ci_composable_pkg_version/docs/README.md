# CI composable package version

Declares `requires.input` on [`ci_input_pkg`](../01_ci_input_pkg/). The test config specifies an explicit `policy.requires` entry with `package: ci_input_pkg` and `version: "0.1.0"` — the dependency is fetched from the registry by pinned name and version, without a local source path override.

**CI:** Built after the stack is up and `package_registry.base_url` points at the local registry (where `ci_input_pkg` has already been built and published).

**Manual:** Build `01_ci_input_pkg` first, start the stack, set `package_registry.base_url` to `https://127.0.0.1:8080`, then build this package.
