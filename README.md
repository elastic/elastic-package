# elastic-package

The elastic-package is a command line tool, written in Go, used for developing Elastic Integrations.

## Features

TODO

## Getting started

Download and build the `elastic-package` binary:

```bash
go get github.com/elastic/elastic-package
```

Change directory to the integration under development

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