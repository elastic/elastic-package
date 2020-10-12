# HOWTO: Writing pipeline tests for a package

## Introduction

Elastic Packages are comprised of data streams. A pipeline test exercises Elasticsearch Ingest Node pipelines defined for a package's data stream.

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

To define a pipeline test we must define configuration at each dataset's level:

```
<package root>/
  data_stream/
    <data stream>/
      _dev/
        test/
          pipeline/
            (test case definitions, both raw files and input events, optional configuration)
      manifest.yml
  manifest.yml
```

### Test case definitions

There are two types of test case definitions - **raw files** and **input events**.

#### Raw files

The raw files simplify preparing test cases using real application `.log` files. A sample log (e.g. `test-access-sample.log`) file may look like the following one for Nginx:

```
127.0.0.1 - - [07/Dec/2016:11:04:37 +0100] "GET /test1 HTTP/1.1" 404 571 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.98 Safari/537.36"
127.0.0.1 - - [07/Dec/2016:11:04:58 +0100] "GET / HTTP/1.1" 304 0 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.12; rv:49.0) Gecko/20100101 Firefox/49.0"
127.0.0.1 - - [07/Dec/2016:11:04:59 +0100] "GET / HTTP/1.1" 304 0 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.12; rv:49.0) Gecko/20100101 Firefox/49.0"
```

The pipeline test runner transforms log lines into file input events and sends to the ingest pipeline. The transformation process can be customized using an optional configuration stored as JSON file with the suffix `-config.json` (e.g. `test-access-sample.log-config.json`):

```json
{
    "multiline": {
        "first_line_pattern": "^(?:[0-9]{1,3}\\.){3}[0-9]{1,3}"
    },
    "fields": {
        "@timestamp": "2020-04-28T11:07:58.223Z",
        "ecs": {
            "version": "1.5.0"
        },
        "event.category": [
            "web"
        ]
    }
}
```

The `multiline` section configures the log file reader to correctly detect multiline log entries using the `first_line_pattern`.

The `fields` section allows for customizing extra fields to be added to every read log entry (e.g. `@timestamp`, `ecs`).

#### Input events

The input events contain mocked JSON events that are ready to be passed to the ingest pipeline as-is. Such events can be helpful in situations in which an input event can't be serialized to a standard log file, e.g. Redis input. A sample file with input events  (e.g. `test-access-event.json`) looks as following:

```json
{
    "events": [
        {
            "@timestamp": "2016-10-25T12:49:34.000Z",
            "message": "127.0.0.1 - - [07/Dec/2016:11:04:37 +0100] \"GET /test1 HTTP/1.1\" 404 571 \"-\" \"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.98 Safari/537.36\"\n"
        },
        {
            "@timestamp": "2016-10-25T12:49:34.000Z",
            "message": "127.0.0.1 - - [07/Dec/2016:11:05:07 +0100] \"GET /taga HTTP/1.1\" 404 169 \"-\" \"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.12; rv:49.0) Gecko/20100101 Firefox/49.0\"\n"
        }
    ]
}
```

#### Expected results

Once the Simulate API processes the given input data, the pipeline test runner will compare them with expected results. Test results are stored as JSON files with the suffix `-expected.json`. A sample test results file is shown below.

```json
{
    "expected": [
        {
            "@timestamp": "2016-12-07T10:04:37.000Z",
            "nginx": {
                "access": {
                    "remote_ip_list": [
                        "127.0.0.1"
                    ]
                }
            },
            ...
        },
        {
            "@timestamp": "2016-12-07T10:05:07.000Z",
            "nginx": {
                "access": {
                    "remote_ip_list": [
                        "127.0.0.1"
                    ]
                }
            },
            ...
        }
    ]
}
```

It's possible to generate the expected test results from the output of the Simulate API. To do so, use the `--generate` switch:

```
elastic-package test pipeline --generate
```

## Running a pipeline test

Once the configurations are defined as described in the previous section, you are ready to run pipeline tests for a package's data streams.

First you must deploy the Elasticsearch instance. This corresponds to step 1 as described in the [_Conceptual process_](#Conceptual-process) section.

```
elastic-package stack up -d --services=elasticsearch
```

For a complete listing of options available for this command, run `elastic-package stack up -h` or `elastic-package help stack up`.

Next, you must set environment variables needed for further `elastic-package` commands.

```
$(elastic-package stack shellinit)
```

Next, you must invoke the pipeline tests runner. This corresponds to steps 2 through 4 as described in the [_Conceptual process_](#Conceptual-process) section.

If you want to run pipeline tests for **all data streams** in a package, navigate to the package's root folder (or any sub-folder under it) and run the following command.

```
elastic-package test pipeline
```

If you want to run pipeline tests for **specific data streams** in a package, navigate to the package's root folder (or any sub-folder under it) and run the following command.

```
elastic-package test pipeline --data-streams <data stream 1>[,<data stream 2>,...]
```

Finally, when you are done running all pipeline tests, bring down the Elastic Stack. This corresponds to step 4 as described in the [_Conceptual process_](#Conceptual-process) section.

```
elastic-package stack down
```
