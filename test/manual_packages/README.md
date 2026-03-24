# Manual Test Packages

Packages under `test/manual_packages/` are **not** picked up by CI build/install scripts (which glob `test/packages/*/*/`). They require manual setup to exercise.

All **`requires.input`** and **variable-merge** fixtures live under [`test/manual_packages/required_inputs/`](required_inputs/). The same trees are used as fixtures by `go test` in [`internal/requiredinputs/variables_test.go`](../../internal/requiredinputs/variables_test.go).

## required_inputs

### Template bundling (smoke)

- `required_inputs/test_input_pkg` — input package; install first.
- `required_inputs/with_input_package_requires` — integration that depends on `test_input_pkg`. Minimal composable setup (no `vars` overrides on the package input).

### Variable merge (composable input vars)

When an integration lists `requires.input` and its policy template references that input package with optional `vars`, elastic-package **merges** variable definitions from the input package into the built manifests (see [`internal/requiredinputs/variables.go`](../../internal/requiredinputs/variables.go) — `mergeVariables`).

| Package | Role |
| --- | --- |
| `required_inputs/var_merging_input_pkg` | Required input package (`paths`, `encoding`, `timeout`). |
| `required_inputs/with_merging_full` | Promoted `paths` + `encoding`; DS merge for `timeout` + novel `custom_tag`. |
| `required_inputs/with_merging_promotes_to_input` | Only `paths` promoted; DS keeps `encoding`, `timeout`. |
| `required_inputs/with_merging_ds_merges` | No promotion; DS merges `encoding` title + adds `custom_tag`. |
| `required_inputs/with_merging_no_override` | No composable overrides; all base vars on DS, unchanged. |
| `required_inputs/with_merging_two_policy_templates` | Two PTs on the same input pkg: one promotes `paths` for its DS only; the other leaves all vars on the DS (`TestMergeVariables_TwoPolicyTemplatesScopedPromotion`). |
| `required_inputs/with_merging_duplicate_error` | Invalid: duplicate `paths` at DS level; **build should fail** with an error mentioning `paths`. |

### Manual testing workflow

1. Start the stack and local package registry:
   ```bash
   elastic-package stack up -d
   ```
2. Configure `package_registry.base_url` in `~/.elastic-package/config.yml` so builds can resolve required input packages (see [local package registry how-to](../../docs/howto/local_package_registry.md) and the root [README](../../README.md) `package_registry` section).
3. Build and install in **dependency order** (input packages before integrations that require them). Examples:

   Template bundling smoke:
   ```bash
   elastic-package build -C test/manual_packages/required_inputs/test_input_pkg --zip
   elastic-package build -C test/manual_packages/required_inputs/with_input_package_requires --zip
   ```

   Variable merge (build `var_merging_input_pkg` first, install it, then build the integration you need):
   ```bash
   elastic-package build -C test/manual_packages/required_inputs/var_merging_input_pkg --zip
   elastic-package build -C test/manual_packages/required_inputs/with_merging_full --zip
   ```

4. Install via the local registry in the same order (e.g. `test_input_pkg` before `with_input_package_requires`; `var_merging_input_pkg` before any `with_merging_*` integration).

For **expected merged manifests** after a successful variable-merge build, see `TestMergeVariables_*` in [`variables_test.go`](../../internal/requiredinputs/variables_test.go). For `with_merging_duplicate_error`, expect `elastic-package build` to fail and the error to contain `paths`.

### When composable inputs are fully supported in CI

Move `required_inputs/` under `test/packages/required_inputs/` so [`scripts/test-build-install-zip.sh`](../../scripts/test-build-install-zip.sh) can build and install them automatically (install order is lexicographic, so `var_merging_input_pkg` is installed before `with_merging_*`). Update [`internal/requiredinputs/variables_test.go`](../../internal/requiredinputs/variables_test.go) fixture paths to match.
