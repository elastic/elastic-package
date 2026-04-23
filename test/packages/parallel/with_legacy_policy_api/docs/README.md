# with_legacy_policy_api

Test package that demonstrates the `policy_api_format: legacy` system test
configuration setting.

This package includes a `select`-type variable (`revoked`) with `"false"` as
one of its option values. Fleet's simplified package policy API has a bug where
`schema.boolean()` runs before `schema.string()` in its validation schema,
causing the JSON string `"false"` to be coerced to the boolean `false`. The
subsequent check `["", "false", "true"].includes(false)` then fails, returning
a 400 error.

The workaround is to set `policy_api_format: legacy` in the system test config,
which makes elastic-package use the legacy arrays-based Fleet API instead of
the simplified objects-based API. The legacy API wraps variable values with
`{"type": ..., "value": ...}` objects, preserving the string type and avoiding
the coercion.
