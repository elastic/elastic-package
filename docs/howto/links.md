# HOWTO: Use links to reuse common files.

## Introduction

Many packages have files that are equal between them. This is more common in pipelines, 
input configurations, and field definitions.

In order to help developers, there is the ability to define links, so a file that might be reused needs to only be defined once, and can be reused from any other packages.


# Links

Currently, there are some specific places where links can be defined:

- `elasticsearch/ingest_pipeline`
- `data_stream/**/elasticsearch/ingest_pipeline`
- `agent/input`
- `data_stream/**/agent/stream`
- `data_stream/**/fields`

A link consists of a file with a `.link` extension that contains a path, relative to its location, to the file that it will be replaced with. It also consists of a checksum to validate the linked file is up to date with the package expectations.

`data_stream/foo/elasticsearch/ingest_pipeline/default.yml.link`

```
../../../../../testpackage/data_stream/test/elasticsearch/ingest_pipeline/default.yml f7c5f0c03aca8ef68c379a62447bdafbf0dcf32b1ff2de143fd6878ee01a91ad
```

This will use the contents of the linked file during validation, tests, and building of the package, so functionally nothing changes from the package point of view.

## The `_dev/shared` folder

As a convenience, shared files can be placed under `_dev/shared` if they are going to be
reused from several places. They can even be added outside of any package, in any place in the repository.

## Managing Links with elastic-package

The `elastic-package` tool provides several subcommands to help manage linked files:

### `elastic-package links check`

Check if all linked files in the current directory and its subdirectories are up to date. This command verifies that the checksums in link files match the actual content of the included files.

```bash
elastic-package links check
```

This command will:
- Scan for all `.link` files in the current directory tree
- Validate that each linked file's checksum matches the included file's current content
- Report any outdated link files that need updating
- Exit with an error if any link files are outdated

### `elastic-package links update`

Update the checksums of all outdated linked files in the current directory and its subdirectories.

```bash
elastic-package links update
```

This command will:
- Find all `.link` files that have outdated checksums
- Calculate new checksums for the included files
- Update the `.link` files with the new checksums
- Report which link files were updated

### `elastic-package links list`

List all packages that have linked files referencing content from the current directory.

```bash
# List all linked file paths
elastic-package links list

# List only package names (without individual file paths)
elastic-package links list --packages
```

This command will:
- Find all `.link` files in the repository that reference files in the current directory
- Group the results by package name
- Display either the full file paths or just the package names (with `--packages` flag)

## Workflow

A typical workflow for managing linked files:

1. **Create a shared file** in a central location (e.g., `_dev/shared/` or in a reference package)

2. **Create link files** in packages that need to reference the shared file:
   ```bash
   echo "../../_dev/shared/common-pipeline.yml" > data_stream/logs/elasticsearch/ingest_pipeline/default.yml.link
   ```

3. **Update checksums** to make the link valid:
   ```bash
   elastic-package links update
   ```

4. **Check links regularly** to ensure they stay up to date:
   ```bash
   elastic-package links check
   ```

5. **When modifying shared files**, update all dependent links:
   ```bash
   # After editing a shared file, update all links that reference it
   elastic-package links update
   ```
