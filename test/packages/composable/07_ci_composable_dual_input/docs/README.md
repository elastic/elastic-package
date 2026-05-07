# CI Composable Dual Input

Declares `requires.input` on both [`05_ci_input_pkg_a`](../05_ci_input_pkg_a/) and [`06_ci_input_pkg_b`](../06_ci_input_pkg_b/), which both declare `input: logfile`. After `elastic-package build` the policy template inputs are disambiguated with `name: ci_input_pkg_a` / `name: ci_input_pkg_b`, and the data stream manifests use those names as their `input:` reference (package-spec SVR00010).

**CI:** Built in a second phase after the stack is up, similar to `02_ci_composable_integration`.

**Manual:** Build both input packages first, start the stack, set `package_registry.base_url` to `https://127.0.0.1:8080`, then build this package.
