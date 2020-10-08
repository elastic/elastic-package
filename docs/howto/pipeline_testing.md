# HOWTO: Writing pipeline tests for a package

## Introduction

Elastic Packages are comprised of data streams. A pipeline test exercises ingest pipelines defined for a package's data stream.

## Conceptual process

Conceptually, running a pipeline test involves the following steps:

1. Deploy the Elasticsearch instance (part of the Elastic Stack). This step takes time so it should typically be done once as a pre-requisite to running pipeline tests on multiple data streams.
1. Upload ingest pipelines to be tested.
1. Use [Simulate API](https://www.elastic.co/guide/en/elasticsearch/reference/master/simulate-pipeline-api.html) to process logs/metrics with the ingest pipeline.
1. Compare generated results with expected ones.

## Limitations

At the moment pipeline tests have limitations. The main ones are:
* Results of simulation can be different from real documents. They depend on the input data which may not have all fields defined.
* There might be integrations which transform data mostly using Beats processors instead of ingest pipelines. In such cases ingest pipelines are rather plain.

## Defining a pipeline test

Packages have a specific folder structure (only relevant parts shown).

```
<package root>/
  data_stream/
    <data stream>/
      manifest.yml
  manifest.yml
```

To define a pipeline test we must define configuration at two levels: the package level and each dataset's level.

### Dataset-level configuration

Next, we must define configuration for each data stream that we want to pipeline test.

```
<package root>/
  data_stream/
    <data stream>/
      _dev/
        test/
          pipeline/
            (test case definitions)
      manifest.yml
  manifest.yml
```

### Test case definitions

There are two types of test case definitions - plain log files and mocked JSON events.

#### Plain log files (.log)

TODO

#### Input events (JSON)

TODO

## Running a pipeline test

Once the configurations is defined as described in the previous section, you are ready to run pipeline tests for a package's data streams.

First you must deploy the Elastic Stack. This corresponds to steps 1 as described in the [_Conceptual process_](#Conceptual-process) section.

```
elastic-package stack up -d
```

For a complete listing of options available for this command, run `elastic-package stack up -h` or `elastic-package help stack up`.

Next, you must set environment variables needed for further `elastic-package` commands.

```
$(elastic-package stack shellinit)
```

Next, you must invoke the pipeline tests runner. This corresponds to steps 2 and 3 as described in the [_Conceptual process_](#Conceptual-process) section.

If you want to run pipeline tests for **all data streams** in a package, navigate to the package's root folder (or any sub-folder under it) and run the following command.

```
elastic-package test pipeline
```

If you want to run pipeline tests for **specific data streams** in a package, navigate to the package's root folder (or any sub-folder under it) and run the following command.

```
elastic-package test pipeline --data-streams <data stream 1>[,<data stream 2>,...]
```

Finally, when you are done running all pipeline tests, bring down the Elastic Stack. This corresponds to step 8 as described in the [_Conceptual process_](#Conceptual_process) section.

```
elastic-package stack down
```