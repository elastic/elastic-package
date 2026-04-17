# Manual test packages

Packages under `test/manual_packages/` are **not** picked up by CI’s main package glob beyond what each script includes. They are for **manual** workflows and **targeted** `go test` cases.

## Composable coverage

End-to-end composable integration coverage (`requires.input`, local registry, build + install) lives under:

- [`composable/01_ci_input_pkg/`](composable/01_ci_input_pkg/) — `type: input` dependency
- [`composable/02_ci_composable_integration/`](composable/02_ci_composable_integration/) — `type: integration` that requires the input package above; must be built after `stack up` with `package_registry.base_url` set to `https://127.0.0.1:8080`

`internal/requiredinputs` integration tests copy those same directories (see `ciInputFixturePath`, `copyComposableIntegrationFixture` in [`variables_test.go`](../../internal/requiredinputs/variables_test.go)).

## `required_inputs` (manual / edge)

Remaining trees under [`required_inputs/`](required_inputs/) exercise **narrow** variable-merge and template cases and are **not** required for the composable CI zip job:

| Package | Role |
| --- | --- |
| `required_inputs/with_merging_promotes_to_input` | Only `paths` promoted; DS keeps `encoding`, `timeout`. |
| `required_inputs/with_merging_ds_merges` | No PT var overrides; DS merges `encoding` title + `custom_tag`. |
| `required_inputs/with_merging_no_override` | No composable overrides; all base vars on DS. |
| `required_inputs/with_merging_two_policy_templates` | Two PTs, scoped promotion on one. |
| `required_inputs/with_merging_duplicate_error` | Invalid duplicate `paths` on DS — **build must fail** (not in CI zip loop). |
| `required_inputs/with_linked_template_path` | Composable + policy `template_path` via `.link` (see [`dependency_management.md`](../../docs/howto/dependency_management.md)). |

All of these depend on **`ci_input_pkg`** from [`composable/01_ci_input_pkg/`](composable/01_ci_input_pkg/) (see each package’s `_dev/test/config.yml` `requires` stub).

### Manual workflow

1. `elastic-package stack up -d`
2. Set `package_registry.base_url` in `~/.elastic-package/config.yml` to `https://127.0.0.1:8080` (see [local package registry how-to](../../docs/howto/local_package_registry.md)).
3. Build and install `01_ci_input_pkg` before any integration that lists `requires.input` for it, then build the integration.

### Expected errors

For `with_merging_duplicate_error`, `elastic-package build` should fail with an error mentioning `paths`.
