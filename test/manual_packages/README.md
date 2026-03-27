# Manual Test Packages

Packages in this directory are **not** picked up by CI build/install scripts (which glob `test/packages/*/*/`). They require manual setup to exercise.

## required_inputs

These packages test the `requires.input` (composable input) feature.

- `required_inputs/test_input_pkg` — the input package that must be installed first.
- `required_inputs/with_input_package_requires` — an integration package that declares a dependency on `test_input_pkg`.

### Manual testing workflow

1. Start the stack and local package registry:
   ```bash
   elastic-package stack up -d
   ```
2. Configure `package_registry.base_url` to point at the stack's registry URL (see `scripts/test-build-install-zip.sh` lines 69–78 for the pattern).
3. Build and install in dependency order:
   ```bash
   elastic-package build -C test/manual_packages/required_inputs/test_input_pkg --zip
   elastic-package build -C test/manual_packages/required_inputs/with_input_package_requires --zip
   ```
4. Install via the local registry, `test_input_pkg` first, then `with_input_package_requires`.

### When composable inputs are fully supported in CI

Move `required_inputs/` back to `test/packages/required_inputs/` so the existing install scripts regain automated coverage without requiring additional special-casing.
