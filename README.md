<!--
WARNING: This is a generated file. Do NOT edit it manually. To regenerate this file, run `make update-readme`.
-->

# elastic-package

`elastic-package` is a command line tool, written in Go, used for developing Elastic packages. It can help you lint, format,
test, build, and promote your packages. Learn about each of these and other features in [_Commands_](#commands) below.

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
make build
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

### `elastic-package build`

_Context: package_

Use this command to build a package. Currently it supports only the "integration" package type.

Built packages are stored in the "build/" folder located at the root folder of the local Git repository checkout that contains your package folder. The command will also render the README file in your package folder if there is a corresponding template file present in "_dev/build/docs/README.md". All "_dev" directories under your package will be omitted.

Built packages are served up by the Elastic Package Registry running locally (see "elastic-package stack"). If you want a local package to be served up by the local Elastic Package Registry, make sure to build that package first using "elastic-package build".

Built packages can also be published to the global package registry service.

For details on how to enable dependency management, see the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/dependency_management.md).

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

### `elastic-package dump`

_Context: global_

Use this command as a exploratory tool to dump assets relevant for the package.

### `elastic-package export`

_Context: package_

Use this command to export assets relevant for the package, e.g. Kibana dashboards.

### `elastic-package format`

_Context: package_

Use this command to format the package files.

The formatter supports JSON and YAML format, and skips "ingest_pipeline" directories as it's hard to correctly format Handlebars template files. Formatted files are being overwritten.

### `elastic-package install`

_Context: package_

Use this command to install the package in Kibana.

The command uses Kibana API to install the package in Kibana. The package must be exposed via the Package Registry.

### `elastic-package lint`

_Context: package_

Use this command to validate the contents of a package using the package specification (see: https://github.com/elastic/package-spec).

The command ensures that the package is aligned with the package spec and the README file is up-to-date with its template (if present).

### `elastic-package profiles`

_Context: global_

Use this command to add, remove, and manage multiple config profiles.
	
Individual user profiles appear in ~/.elastic-package/stack, and contain all the config files needed by the "stack" subcommand. 
Once a new profile is created, it can be specified with the -p flag, or the ELASTIC_PACKAGE_PROFILE environment variable.
User profiles are not overwritten on upgrade of elastic-stack, and can be freely modified to allow for different stack configs.

### `elastic-package promote`

_Context: global_

Use this command to move packages between the snapshot, staging, and production stages of the package registry.

This command is intended primarily for use by administrators.

It allows for selecting packages for promotion and opens new pull requests to review changes. Please be aware that the tool checks out an in-memory Git repository and switches over branches (snapshot, staging and production), so it may take longer to promote a larger number of packages.

### `elastic-package publish`

_Context: package_

Use this command to publish a new package revision.

The command checks if the package hasn't been already published (whether it's present in snapshot/staging/production branch or open as pull request). If the package revision hasn't been published, it will open a new pull request.

### `elastic-package service`

_Context: package_

Use this command to boot up the service stack that can be observed with the package.

The command manages lifecycle of the service stack defined for the package ("_dev/deploy") for package development and testing purposes.

### `elastic-package stack`

_Context: global_

Use this command to spin up a Docker-based Elastic Stack consisting of Elasticsearch, Kibana, and the Package Registry. By default the latest released version of the stack is spun up but it is possible to specify a different version, including SNAPSHOT versions.

Be aware that a common issue while trying to boot up the stack is that your Docker environments settings are too low in terms of memory threshold.

For details on how to connect the service with the Elastic stack, see the [service command](https://github.com/elastic/elastic-package/blob/main/README.md#elastic-package-service).

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

### `elastic-package uninstall`

_Context: package_

Use this command to uninstall the package in Kibana.

The command uses Kibana API to uninstall the package in Kibana. The package must be exposed via the Package Registry.

### `elastic-package version`

_Context: global_

Use this command to print the version of elastic-package that you have installed. This is especially useful when reporting bugs.



### GitHub authorization

The `promote` and `publish` commands require access to the GitHub API to open pull requests or check authorized account data.
The tool uses the GitHub token to authorize user's call to API. The token can be stored in the `~/.elastic/github.token`
file or passed via the `GITHUB_TOKEN` environment variable.

Here are the instructions on how to create your own personal access token (PAT):
https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token

Make sure you have enabled the following scopes:
* `public_repo` — to open pull requests on GitHub repositories.
* `read:user` and `user:email` — to read your user profile information from GitHub in order to populate pull requests appropriately.

After creating or modifying your personal access token, authorize the token for
use of the Elastic organization: https://docs.github.com/en/github/authenticating-to-github/authenticating-with-saml-single-sign-on/authorizing-a-personal-access-token-for-use-with-saml-single-sign-on

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
