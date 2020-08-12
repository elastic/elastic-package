# elastic-package

The elastic-package is a command line tool, written in Go, used for developing Elastic packages.

*For experimental use only*

## Features

TODO

## Supported package types

* [Elastic Integrations](https://github.com/elastic/integrations)

## Getting started

Download and build the latest master of `elastic-package` binary:

```bash
go get github.com/elastic/elastic-package
```

Change directory to the package under development. Note: an integration is a specific type of a package.

```bash
cd integrations
cd package/my-integration
```

Run the `help` command and see available actions:

```bash
elastic-package help
```

## Development

Even though the project is "go-gettable", there is the `Makefile` present, which can be used to build, format or vendor
source code:

`make build` - build the tool source

`make format` - format the Go code

`make vendor` - vendor code of dependencies

`make check` - one-liner, used by CI to verify if source code is ready to be pushed to the repository