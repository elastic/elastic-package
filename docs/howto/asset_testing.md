# HOWTO: Running asset loading tests for a package

## Introduction

Elastic Packages define assets to be loaded into Elasticsearch and Kibana. Asset loading tests exercise installing a package to ensure that its assets are loaded into Elasticsearch and Kibana as expected.

## Conceptual process

Conceptually, running an asset load test involves the following steps:

1. Build the package.
1. Deploy Elasticsearch, Kibana, and the Package Registry (all part of the Elastic Stack). This step takes time so it should typically be done once as a pre-requisite to running asset loading tests on multiple packages.
1. Install the package.
1. Use various Kibana and Elasticsearch APIs to assert that the package's assets were loaded into Kibana and Elasticsearch as expected.
1. Remove the package.

## Defining an asset loading test

As a package developer, you do not need to do any work to define an asset loading test for your package. All the necessary information is already present in the package's files.

## Running an asset loading test

First, you must build your package. This corresponds to step 1 as described in the [_Conceptual process_](#Conceptual-process) section.

Navigate to the package's root folder (or any sub-folder under it) and run the following command.

```
elastic-package build
```

Next, you must deploy Elasticsearch, Kibana, and the Package Registry. This corresponds to step 2 as described in the [_Conceptual process_](#Conceptual-process) section.

```
elastic-package stack up -d
```

For a complete listing of options available for this command, run `elastic-package stack up -h` or `elastic-package help stack up`.

Next, you must invoke the asset loading test runner. This corresponds to steps 3 through 5 as described in the [_Conceptual process_](#Conceptual-process) section.

Navigate to the package's root folder (or any sub-folder under it) and run the following command.

```
elastic-package test asset
```

Finally, when you are done running all asset loading tests, bring down the Elastic Stack. This corresponds to step 4 as described in the [_Conceptual process_](#Conceptual-process) section.

```
elastic-package stack down
```

## Global test configuration

Each package could define a configuration file in `_dev/test/config.yml` to skip all the asset tests.

```yaml
asset:
  skip:
    reason: <reason>
    link: <link_to_issue>
```