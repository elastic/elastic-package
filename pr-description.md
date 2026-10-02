## Summary

- Add `"blueprint"` to `AllowedPackageTypes` so package-root detection recognizes blueprint packages. This unblocks `format`, `lint`, `build`, and `check` from failing early with `package root not found`.
- Teach `elastic-package create package` to scaffold blueprint packages (wizard + `--type blueprint`): sample IaC under `blueprints/`, and omit `img/`, `_dev/`, categories, icons, and screenshots to match the upcoming package-spec layout.
- Treat `blueprint` like other non-data-stream package types in `PackageHasDataStreams`.
- Add unit/archetype coverage and a minimal fixture under `internal/packages/testdata/blueprint`.

Closes #3827

Relates to elastic/package-spec#1209 (blueprint package type / dynamic IaC).

## Context

A new `blueprint` package type is being introduced in package-spec for canonical IaC base templates. Without recognizing that type in `AllowedPackageTypes`, every core command fails before doing real work. This is also a prerequisite for the integrations repo publish pipeline once blueprint packages start landing there.

## Test plan

- [x] `go test ./internal/packages/ ./internal/packages/archetype/ ./cmd/ -run 'TestAllowedPackageTypes|TestFindPackageRoot|TestPackage|TestCreatePackage'`
- [x] `elastic-package create package --type blueprint --name <tmp>` creates a valid-looking package root (manifest type `blueprint`, `blueprints/` present, no `img/`/`_dev`)
- [x] `elastic-package format -C internal/packages/testdata/blueprint` succeeds
- [x] `make build format lint licenser gomod` (with `CGO_ENABLED=0` for `make update` if needed in local env)
- [ ] Confirm `lint` / `build` / `check` succeed against the fixture after bumping to a package-spec release that includes the `blueprint` type (currently blocked on package-spec#1209; verified locally against that PR tip via a temporary replace)

## Follow-ups

- Bump `github.com/elastic/package-spec/v3` once the blueprint type ships, then re-run `lint`/`build`/`check` on the fixture (and optionally move/add a package under `test/packages/` for CI coverage).
- Decide whether baseline semantic validation (changelog link, version integrity, etc.) should be wired for `blueprint` before GA.
