# HOWTO: Create new packages and data streams

## Introduction

The `elastic-package` tool can be used to bootstrap a new package or add a data stream using an embedded archetype ([resource templates](https://github.com/elastic/elastic-package/tree/main/internal/packages/archetype)).
It's advised to use `elastic-package create` to build new package rather than copying sources of an existing package.
This will ensure that you're following latest recommendations for the package format.

## Create new package

### Prerequisites

_Pick the directory where you'd like to create a new package. For integrations, it's: [packages/](https://github.com/elastic/integrations/tree/master/packages)._

_Decide what [package type](./package_types.md) you would like to create._

### Steps

1. Bootstrap new package using the TUI wizard: `elastic-package create package`.
2. Adjust the created package manually:
    * define policy templates and inputs
    * add icons and screenshots
    * update README files
    * update `changelog.yml` file
3. Verify the package:
    1. Enter the package directory: `cd <new_package>`.
    2. Check package correctness: `elastic-package check`.

## Add data stream

### Prerequisites

_Enter the package directory. For nginx integration, it's: [packages/nginx/](https://github.com/elastic/integrations/tree/master/packages/nginx)._

### Steps

1. Bootstrap new data stream using the TUI wizard: `elastic-package create data-stream`.
2. Adjust the created data stream manually:
    * define streams and required vars
    * define used fields
    * define ingest pipeline definition (if necessary)
    * update the agent's stream configuration
3. Verify the package:
    1. Enter the package directory: `cd <new_package>`.
    2. Check package correctness: `elastic-package check`.

## Export package dashboards

Once the package assets are defined, these should be exported using the `elastic-package export` command. As dashboards are the only type of exportable asset, the command is:
```
elastic-package export dashboards
```

Used this way, this command will open an interactive prompt for dashboard selection. Alternatively, a comma-separated list of dashboard ids can be provided with the `-d` flag:
```
elastic-package export dashboards -d 123,345,789
```

### Edit package dashboards

When updating an existing package, assets may need to be updated, e.g. to make use of recently added features. Dashboards can be made editable by using the [`elastic-package edit dashboards` command](https://github.com/elastic/elastic-package/blob/main/docs/howto/make_dashboards_editable.md).
