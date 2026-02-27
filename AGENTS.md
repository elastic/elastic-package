# Agent Instructions for elastic-package

## Build & Test

```bash
go build ./...
go test ./...
```

Run tests for specific packages:
```bash
go test ./internal/packages/... ./internal/kibana/... ./internal/resources/... ./internal/testrunner/...
```

Run integration tests with test packages:
```bash
go run . stack up -v -d
go run . test -C ./test/packages/parallel/apache
```

*Important*: Linting before giving the task for complete.
```bash
make build format lint licenser gomod update
```

## Code Conventions

### Go Style
- Keep exported surface small — unexport functions that are only used within the same package.
- Do not leave trailing spaces on new lines.
- Do not leave trailing newlines at the end of files.

### Error Handling
- Wrap errors with context using `fmt.Errorf("...: %w", err)`.
- Return explicit errors rather than silently falling back to a default value.

### Function Placement
- Place functions in the package that owns the types they primarily operate on.
- Functions that produce `kibana.*` types and call `kibana.*` helpers belong in `internal/kibana`.
- Functions that navigate or filter `packages.*` types belong in `internal/packages`.
- Resource lifecycle logic (create/update/delete) belongs in `internal/resources`.
- Disk I/O should happen at the call site, not inside builder/helper functions that are meant to be pure transformations over already-loaded data.

## Internal Package Import Hierarchy

The import graph must be acyclic. The established layering is:

```
internal/packages
    ↑
internal/kibana       (imports packages)
    ↑
internal/resources    (imports kibana, packages)
    ↑
internal/testrunner   (imports resources, kibana, packages)
```

## Tests

- When possible use real package fixtures from `test/packages/` or `testdata` directories rather than inline YAML strings. Refer to them with relative paths like `../../test/packages/...`.

## Fleet Package Policy API

elastic-package uses the objects-based Fleet API (`PackagePolicy`) — not the deprecated arrays-based API (`PackageDataStream`).

### Key concepts
- **Input key**: `"{policyTemplate.Name}-{input.Type}"` (e.g. `"apache-logfile"`).
- **Stream key**: built by `datasetKey(pkgName, ds)` — uses `ds.Dataset` when set, otherwise `"{pkgName}.{ds.Name}"`.
- **Sibling stream disabling**: Fleet auto-enables all streams for an enabled input unless they are explicitly listed with `enabled: false`. Always send `{enabled: false}` for every sibling data stream sharing the same input type within the same policy template.
- **Policy template scoping**: When a policy template declares a `data_streams` list, only include data streams from that list as siblings. Use `packages.DataStreamsForInput(packageRoot, policyTemplate, streamInput)` to get the correct set.
- **Variable format**: the objects-based API expects raw values, not `{"type": ..., "value": ...}` wrappers. `Vars.ToMapStr()` extracts raw values via `val.Value.Value()`.
