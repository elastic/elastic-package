# HOWTO: Running static tests for a package

## Introduction

Static tests allow you to verify if all static resources of the package are valid, e.g. are all fields of the `sample_event.json` documented.
They don't require any additional configuration (unless you would like to skip them).

## Coverage

Static tests cover the following resources:

1. Sample event for a data stream - verification if the file uses only documented fields. 

## Running static tests

Static tests don't require the Elastic stack to be up and running. Simply navigate to the package's root folder
(or any sub-folder under it) and run the following command.

```
elastic-package test static
```

If you want to run pipeline tests for **specific data streams** in a package, navigate to the package's root folder
(or any sub-folder under it) and run the following command.

```
elastic-package test static --data-streams <data stream 1>[,<data stream 2>,...]
```

## Global test configuration

Each package could define a configuration file in `_dev/test/config.yml` to skip all the static tests.

```yaml
static:
  skip:
    reason: <reason>
    link: <link_to_issue>
```