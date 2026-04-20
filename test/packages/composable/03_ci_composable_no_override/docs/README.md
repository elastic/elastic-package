# CI composable no override

Declares `requires.input` on [`ci_input_pkg`](../01_ci_input_pkg/). The test config has **no source override** — the dependency is resolved from the local registry automatically based on the manifest `requires.input` declaration.

**CI:** Built after the stack is up and `package_registry.base_url` points at the local registry (where `ci_input_pkg` has already been built and published).

**Manual:** Build `01_ci_input_pkg` first, start the stack, set `package_registry.base_url` to `https://127.0.0.1:8080`, then build this package.
