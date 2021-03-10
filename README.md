# elastic-package

`elastic-package` is a command line tool, written in Go, used for developing Elastic packages. It can help you lint, format, 
test, build, and promote your packages. Learn about each of these and other features in [_Commands_](#commands) below.

Currently, `elastic-package` only supports packages of type [Elastic Integrations](https://github.com/elastic/integrations).

## Getting started

Download and build the latest master of `elastic-package` binary:

```bash
git clone https://github.com/elastic/elastic-package.git
make build
```

Alternatively, you may use `go get` but you will not be able to use the `elastic-package version` command.

```bash
go get github.com/elastic/elastic-package
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


### `elastic-package build`

_Context: package_

Use this command to build a package. Built packages are stored in the `build/` folder located at the root folder of the local Git repository checkout that contains your package folder.

Built packages are served up by the Elastic Package Registry running locally (see 
`elastic-package stack`). If you want a local package to be served up by the local
Elastic Package Registry, make sure to build that package _first_ using 
`elastic-package build`.

Built packages can also be published to the `package-storage` repository.


### `elastic-package check`

_Context: package_

Use this command to run the `format`, `lint`, and `build` commands all at once, in that order.


### `elastic-package clean`

_Context: package_

Use this command to clean resources used for building the package.


### `elastic-package format`

_Context: package_

Use this command to format the contents of a package.


### `elastic-package install`

_Context: package_

Use this command to install the package in Kibana.


### `elastic-package lint`

_Context: package_

Use this command to validate the contents of a package using the 
[package specification](https://github.com/elastic/package-spec).


### `elastic-package export`

_Context: package_

Use this command to export assets relevant for the package, e.g. Kibana dashboards.


### `elastic-package promote`

_Context: global_

Use this command to promote packages from one stage of the Package Registry to another.

:warning: This command is intended primarily for use by administrators. 

#### GitHub authorization

The `promote` command requires access to the GitHub API to open pull requests or check authorized account data.
The tool uses the GitHub token to authorize user's call to API. The token can be stored in the `~/.elastic/github.token`
file or passed via the `GITHUB_TOKEN` environment variable.

Here are the instructions on how to create your own personal access token (PAT):
https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token

Make sure you have enabled the following scopes:
* `public_repo` — to open pull requests on GitHub repositories.
* `read:user` and `user:email` — to read your user profile information from GitHub in order to populate pull requests appropriately.


### `elastic-package stack`

_Context: global_

Use this command to spin up a Docker-based Elastic Stack consisting of Elasticsearch, Kibana, and 
the Package Registry. By default the latest released version of the stack is spun up but it is possible
to specify a different version, including SNAPSHOT versions.

For details on how to connect the service with the Elastic stack, see the [HOWTO guide](docs/howto/connect_service_with_elastic_stack.md).


### `elastic-package test`

_Context: package_

Use this command to run tests on a package. Currently, there are two types of tests available.

#### Asset Loading Tests

These tests ensure that all the Elasticsearch and Kibana assets defined by your package get loaded up as expected.

For details on how to run asset loading tests for a package, see the [HOWTO guide](docs/howto/asset_testing.md).

#### Pipeline Tests

These tests allow you to exercise any Ingest Node Pipelines defined by your packages.

For details on how to configure and run pipeline tests for a package, see the [HOWTO guide](docs/howto/pipeline_testing.md).

#### Static Tests

These tests allow you to verify if all static resources of the package are valid, e.g. if all fields of the `sample_event.json` are documented.

For details on how to run static tests for a package, see the [HOWTO guide](docs/howto/static_testing.md).

#### System Tests

These tests allow you to test a package's ability to ingest data end-to-end. 

For details on how to configure and run system tests for a package, see the [HOWTO guide](docs/howto/system_testing.md).


### `elastic-package uninstall`

_Context: package_

Use this command to uninstall the package from Kibana.


### `elastic-package version`

_Context: global_

Use this command to print the version of `elastic-package` that you have installed. This is
especially useful when reporting bugs.


## Development

Even though the project is "go-gettable", there is the `Makefile` present, which can be used to build, format or vendor
source code:

`make build` - build the tool source

`make format` - format the Go code

`make vendor` - vendor code of dependencies

`make check` - one-liner, used by CI to verify if source code is ready to be pushed to the repository
