<!--
WARNING: This is a generated file. Do NOT edit it manually. To regenerate this file, run `make update-readme`.
-->

# elastic-package

`elastic-package` is a command line tool, written in Go, used for developing Elastic packages. It can help you lint, format,
test and build your packages. Learn about each of these and other features in [_Commands_](#commands) below.

Currently, `elastic-package` only supports packages of type [Elastic Integrations](https://github.com/elastic/integrations).

## Getting started

Download latest release from the [Releases](https://github.com/elastic/elastic-package/releases/latest) page.

On macOS, use `xattr -r -d com.apple.quarantine elastic-package` after downloading to allow the binary to run.

Alternatively, you may use `go install` but you will not be able to use the `elastic-package version` command or check updates.

```bash
go install github.com/elastic/elastic-package@latest
```

_Please make sure that you've correctly [setup environment variables](https://golang.org/doc/gopath_code.html#GOPATH) -
`$GOPATH` and `$PATH`, and `elastic-package` is accessible from your `$PATH`._

Change directory to the package under development.

```bash
cd my-package
```

Run the `help` command and see available commands:

```bash
elastic-package help
```

## Development

Download and build the latest main of `elastic-package` binary:

```bash
git clone https://github.com/elastic/elastic-package.git
cd elastic-package
make build
```

When developing on Windows, please use the `core.autocrlf=input` or `core.autocrlf=false` option to avoid issues with CRLF line endings:
```bash
git clone --config core.autocrlf=input https://github.com/elastic/elastic-package.git
cd elastic-package
make build
```

This option can be also configured on existing clones with the following commands. Be aware that these commands
will remove uncommited changes.
```bash
git config core.autocrlf input
git rm --cached -r .
git reset --hard
```

## Commands

`elastic-package` currently offers the commands listed below.

Some commands have a _global context_, meaning that they can be executed from anywhere and they will have the
same result. Other commands have a _package context_; these must be executed from somewhere under a package's
root folder and they will operate on the contents of that package.

For more details on a specific command, run `elastic-package help <command>`.

### `elastic-package help`

_Context: global_

Use this command to get a listing of all commands available under `elastic-package` and a brief
description of what each command does.

### `elastic-package completion`

_Context: global_

Use this command to output shell completion information.

The command output shell completions information (for `bash`, `zsh`, `fish` and `powershell`). The output can be sourced in the shell to enable command completion.

Run `elastic-package completion` and follow the instruction for your shell.

### `elastic-package benchmark`

_Context: package_

Use this command to run benchmarks on a package. Currently, the following types of benchmarks are available:

#### Pipeline Benchmarks

These benchmarks allow you to benchmark any Ingest Node Pipelines defined by your packages.

For details on how to configure pipeline benchmarks for a package, review the [HOWTO guide](./docs/howto/pipeline_benchmarking.md).

#### System Benchmarks

These benchmarks allow you to benchmark an integration end to end.

For details on how to configure system benchmarks for a package, review the [HOWTO guide](./docs/howto/system_benchmarking.md).

### `elastic-package benchmark generate-corpus`

_Context: package_


*BEWARE*: this command is in beta and it's behaviour may change in the future.
Use this command to generate benchmarks corpus data for a package.
Currently, only data for what we have related assets on https://github.com/elastic/elastic-integration-corpus-generator-tool are supported.
For details on how to run this command, review the [HOWTO guide](./docs/howto/generate_corpus.md).

### `elastic-package benchmark pipeline`

_Context: package_

Run pipeline benchmarks for the package.

### `elastic-package benchmark system`

_Context: package_

Run system benchmarks for the package.

### `elastic-package build`

_Context: package_

Use this command to build a package. Currently it supports only the "integration" package type.

Built packages are stored in the "build/" folder located at the root folder of the local Git repository checkout that contains your package folder. The command will also render the README file in your package folder if there is a corresponding template file present in "_dev/build/docs/README.md". All "_dev" directories under your package will be omitted. For details on how to generate and syntax of this README, see the [HOWTO guide](./docs/howto/add_package_readme.md).

Built packages are served up by the Elastic Package Registry running locally (see "elastic-package stack"). If you want a local package to be served up by the local Elastic Package Registry, make sure to build that package first using "elastic-package build".

Built packages can also be published to the global package registry service.

For details on how to enable dependency management, see the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/dependency_management.md).

### `elastic-package changelog`

_Context: package_

Use this command to work with the changelog of the package.

You can use this command to modify the changelog following the expected format and good practices.
This can be useful when introducing changelog entries for changes done by automated processes.


### `elastic-package changelog add`

_Context: package_

Use this command to add an entry to the changelog file.

The entry added will include the given description, type and link. It is added on top of the
last entry in the current version

Alternatively, you can start a new version indicating the specific version, or if it should
be the next major, minor or patch version.

### `elastic-package check`

_Context: package_

Use this command to verify if the package is correct in terms of formatting, validation and building.

It will execute the format, lint, and build commands all at once, in that order.

### `elastic-package clean`

_Context: package_

Use this command to clean resources used for building the package.

The command will remove built package files (in build/), files needed for managing the development stack (in ~/.elastic-package/stack/development) and stack service logs (in ~/.elastic-package/tmp/service_logs).

### `elastic-package create`

_Context: global_

Use this command to create a new package or add more data streams.

The command can help bootstrap the first draft of a package using embedded package template. It can be used to extend the package with more data streams.

For details on how to create a new package, review the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/create_new_package.md).

### `elastic-package create data-stream`

_Context: global_

Use this command to create a new data stream.

The command can extend the package with a new data stream using embedded data stream template and wizard.

### `elastic-package create package`

_Context: global_

Use this command to create a new package.

The command can bootstrap the first draft of a package using embedded package template and wizard.

### `elastic-package dump`

_Context: global_

Use this command as an exploratory tool to dump resources from Elastic Stack (objects installed as part of package and agent policies).

### `elastic-package dump agent-policies`

_Context: global_

Use this command to dump agent policies created by Fleet as part of a package installation.

Use this command as an exploratory tool to dump agent policies as they are created by Fleet when installing a package. Dumped agent policies are stored in files as they are returned by APIs of the stack, without any processing.

If no flag is provided, by default this command dumps all agent policies created by Fleet.

If --package flag is provided, this command dumps all agent policies that the given package has been assigned to it.

### `elastic-package dump installed-objects`

_Context: global_

Use this command to dump objects installed by Fleet as part of a package.

Use this command as an exploratory tool to dump objects as they are installed by Fleet when installing a package. Dumped objects are stored in files as they are returned by APIs of the stack, without any processing.

### `elastic-package export`

_Context: package_

Use this command to export assets relevant for the package, e.g. Kibana dashboards.

### `elastic-package export dashboards`

_Context: package_

Use this command to export dashboards with referenced objects from the Kibana instance.

Use this command to download selected dashboards and other associated saved objects from Kibana. This command adjusts the downloaded saved objects according to package naming conventions (prefixes, unique IDs) and writes them locally into folders corresponding to saved object types (dashboard, visualization, map, etc.).

### `elastic-package format`

_Context: package_

Use this command to format the package files.

The formatter supports JSON and YAML format, and skips "ingest_pipeline" directories as it's hard to correctly format Handlebars template files. Formatted files are being overwritten.

### `elastic-package install`

_Context: package_

Use this command to install the package in Kibana.

The command uses Kibana API to install the package in Kibana. The package must be exposed via the Package Registry or built locally in zip format so they can be installed using --zip parameter. Zip packages can be installed directly in Kibana >= 8.7.0. More details in this [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/install_package.md).

### `elastic-package lint`

_Context: package_

Use this command to validate the contents of a package using the package specification (see: https://github.com/elastic/package-spec).

The command ensures that the package is aligned with the package spec and the README file is up-to-date with its template (if present).

### `elastic-package profiles`

_Context: global_

Use this command to add, remove, and manage multiple config profiles.

Individual user profiles appear in ~/.elastic-package/stack, and contain all the config files needed by the "stack" subcommand.
Once a new profile is created, it can be specified with the -p flag, or the ELASTIC_PACKAGE_PROFILE environment variable.
User profiles can be configured with a "config.yml" file in the profile directory.

### `elastic-package profiles create`

_Context: global_

Create a new profile.

### `elastic-package profiles delete`

_Context: global_

Delete a profile.

### `elastic-package profiles list`

_Context: global_

List available profiles.

### `elastic-package profiles use`

_Context: global_

Sets the profile to use when no other is specified.

### `elastic-package promote`

_Context: global_

[DEPRECATED] Use this command to move packages between the snapshot, staging, and production stages of the package registry.

This command is intended primarily for use by administrators.

It allows for selecting packages for promotion and opens new pull requests to review changes. Please be aware that the tool checks out an in-memory Git repository and switches over branches (snapshot, staging and production), so it may take longer to promote a larger number of packages.

### `elastic-package publish`

_Context: package_

[DEPRECATED] Use this command to publish a new package revision.

The command checks if the package hasn't been already published (whether it's present in snapshot/staging/production branch or open as pull request). If the package revision hasn't been published, it will open a new pull request.

### `elastic-package report`

_Context: package_

Use this command to generate various reports relative to the packages. Currently, the following types of reports are available:

#### Benchmark report for Github

These report will be generated by comparing local benchmark results against ones from another benchmark run.
The report will show performance differences between both runs.

It is formatted as a Markdown Github comment to use as part of the CI results.


### `elastic-package report benchmark`

_Context: package_

Generate a benchmark report comparing local results against ones from another benchmark run.

### `elastic-package service`

_Context: package_

Use this command to boot up the service stack that can be observed with the package.

The command manages lifecycle of the service stack defined for the package ("_dev/deploy") for package development and testing purposes.

### `elastic-package service up`

_Context: package_

Boot up the stack.

### `elastic-package stack`

_Context: global_

Use this command to spin up a Docker-based Elastic Stack consisting of Elasticsearch, Kibana, and the Package Registry. By default the latest released version of the stack is spun up but it is possible to specify a different version, including SNAPSHOT versions by appending --version <version>.

You can run your own custom images for Elasticsearch, Kibana or Elastic Agent, see [this document](./docs/howto/custom_images.md).

Be aware that a common issue while trying to boot up the stack is that your Docker environments settings are too low in terms of memory threshold.

For details on how to connect the service with the Elastic stack, see the [service command](https://github.com/elastic/elastic-package/blob/main/README.md#elastic-package-service).

### `elastic-package stack down`

_Context: global_

Take down the stack.

### `elastic-package stack dump`

_Context: global_

Dump stack data for debug purposes.

### `elastic-package stack shellinit`

_Context: global_

Export environment variables.

### `elastic-package stack status`

_Context: global_

Show status of the stack services.

### `elastic-package stack up`

_Context: global_

Use this command to boot up the stack locally.

By default the latest released version of the stack is spun up but it is possible to specify a different version, including SNAPSHOT versions by appending --version <version>.

You can run your own custom images for Elasticsearch, Kibana or Elastic Agent, see [this document](./docs/howto/custom_images.md).

Be aware that a common issue while trying to boot up the stack is that your Docker environments settings are too low in terms of memory threshold.

To expose local packages in the Package Registry, build them first and boot up the stack from inside of the Git repository containing the package (e.g. elastic/integrations). They will be copied to the development stack (~/.elastic-package/stack/development) and used to build a custom Docker image of the Package Registry. Starting with Elastic stack version >= 8.7.0, it is not mandatory to be available local packages in the Package Registry to run the tests.

For details on how to connect the service with the Elastic stack, see the [service command](https://github.com/elastic/elastic-package/blob/main/README.md#elastic-package-service).

### `elastic-package stack update`

_Context: global_

Update the stack to the most recent versions.

### `elastic-package status [package]`

_Context: package_

Use this command to display the current deployment status of a package.

If a package name is specified, then information about that package is
returned, otherwise this command checks if the current directory is a
package directory and reports its status.

### `elastic-package test`

_Context: package_

Use this command to run tests on a package. Currently, the following types of tests are available:

#### Asset Loading Tests
These tests ensure that all the Elasticsearch and Kibana assets defined by your package get loaded up as expected.

For details on how to run asset loading tests for a package, see the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/asset_testing.md).

#### Pipeline Tests
These tests allow you to exercise any Ingest Node Pipelines defined by your packages.

For details on how to configure pipeline test for a package, review the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/pipeline_testing.md).

#### Static Tests
These tests allow you to verify if all static resources of the package are valid, e.g. if all fields of the sample_event.json are documented.

For details on how to run static tests for a package, see the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/static_testing.md).

#### System Tests
These tests allow you to test a package's ability to ingest data end-to-end.

For details on how to configure amd run system tests, review the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/system_testing.md).

### `elastic-package test asset`

_Context: package_

Run asset loading tests for the package.

### `elastic-package test pipeline`

_Context: package_

Run pipeline tests for the package.

### `elastic-package test static`

_Context: package_

Run static files tests for the package.

### `elastic-package test system`

_Context: package_

Run system tests for the package.

### `elastic-package uninstall`

_Context: package_

Use this command to uninstall the package in Kibana.

The command uses Kibana API to uninstall the package in Kibana. The package must be exposed via the Package Registry.

### `elastic-package version`

_Context: global_

Use this command to print the version of elastic-package that you have installed. This is especially useful when reporting bugs.



## Elastic Package profiles

The `profiles` subcommand allows to work with different configurations. By default,
`elastic-package` uses the "default" profile. Other profiles can be created with the
`elastic-package profiles create` command. Once a profile is created, it will have its
own directory inside the elastic-package data directory. Once you have more profiles,
you can change the default with `elastic-package profiles use`.

You can find the profiles in your system with `elastic-package profiles list`.

You can delete profiles with `elastic-package profiles delete`.

Each profile can have a `config.yml` file that allows to persist configuration settings
that apply only to commands using this profile. You can find a `config.yml.example` that
you can copy to start.

The following settings are available per profile:

* `stack.geoip_dir` defines a directory with GeoIP databases that can be used by
  Elasticsearch in stacks managed by elastic-package. It is recommended to use
  an absolute path, out of the `.elastic-package` directory.

## Development

Even though the project is "go-gettable", there is the `Makefile` present, which can be used to build, format or vendor
source code:

`make build` - build the tool source

`make format` - format the Go code

`make install` - build the tool source and move binary to `$GOBIN`

`make vendor` - vendor code of dependencies

`make check` - one-liner, used by CI to verify if source code is ready to be pushed to the repository

## Release process

This project uses [GoReleaser](https://goreleaser.com/) to release a new version of the application (semver). Release publishing
is automatically managed by the Jenkins CI ([Jenkinsfile](https://github.com/elastic/elastic-package/blob/main/.ci/Jenkinsfile))
and it's triggered by Git tags. Release artifacts are available in the [Releases](https://github.com/elastic/elastic-package/releases) section.

### Steps to create a new release

1. Fetch latest main from upstream (remember to rebase the branch):

```bash
git fetch upstream
git rebase upstream/main
```

2. Create Git tag with release candidate:

```bash
git tag v0.15.0 # let's release v0.15.0!
```

3. Push new tag to the upstream.

```bash
git push upstream v0.15.0
```

The CI will run a new job for the just pushed tag and publish released artifacts. Please expect an automated follow-up PR
in the [Integrations](https://github.com/elastic/integrations) repository to bump up the version ([sample PR](https://github.com/elastic/integrations/pull/1516)).
