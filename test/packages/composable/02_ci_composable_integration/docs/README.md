# CI composable integration

Declares `requires.input` on [`ci_input_pkg`](../01_ci_input_pkg/). After `elastic-package build` with the input package available from the registry, the built package includes merged variables, bundled input templates and fields, and resolved input types.

**CI:** Built in a second phase by `scripts/test-build-install-zip.sh` after the stack is up and `package_registry.base_url` points at the local registry.

**Manual:** Build `01_ci_input_pkg` first, start the stack, set `package_registry.base_url` to `https://127.0.0.1:8080`, then build this package.
