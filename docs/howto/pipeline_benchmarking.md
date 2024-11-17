# HOWTO: Writing pipeline benchmarks for a package

## Introduction

Elastic Packages are comprised of data streams. A pipeline benchmark exercises Elasticsearch Ingest Node pipelines defined for a package's data stream.

## Conceptual process

Conceptually, running a pipeline benchmark involves the following steps:

1. Deploy the Elasticsearch instance (part of the Elastic Stack). This step takes time so it should typically be done once as a pre-requisite to running pipeline benchmarks on multiple data streams.
1. Upload ingest pipelines to be benchmarked.
1. Use [Simulate API](https://www.elastic.co/guide/en/elasticsearch/reference/master/simulate-pipeline-api.html) to process logs/metrics with the ingest pipeline.
1. Gather statistics of the involved processors and show them in a report.

## Limitations

At the moment pipeline benchmarks have limitations. The main ones are:
* As you're only benchmarking the ingest pipeline, you can prepare mocked documents with imaginary fields, different from ones collected in Beats. Also the other way round, you can skip most of the processors and as examples use tiny documents with minimal set of fields just to run the processing simulation.
* There might be integrations which transform data mostly using Beats processors instead of ingest pipelines. In such cases ingest pipeline benchmarks are rather plain.

## Defining a pipeline benchmark

Packages have a specific folder structure (only relevant parts shown).

```
<package root>/
  data_stream/
    <data stream>/
      manifest.yml
  manifest.yml
```

To define a pipeline benchmark we must define configuration at each dataset's level:

```
<package root>/
  data_stream/
    <data stream>/
      _dev/
        benchmark/
          pipeline/
            (benchmark samples definitions, both raw files and input events, optional configuration)
      manifest.yml
  manifest.yml
```

### Benchmark definitions

There are two types of benchmark samples definitions - **raw files** and **input events**.

#### Raw files

The raw files simplify preparing samples using real application `.log` files. A sample log (e.g. `access-sample.log`) file may look like the following one for Nginx:

```
127.0.0.1 - - [07/Dec/2016:11:04:37 +0100] "GET /test1 HTTP/1.1" 404 571 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.98 Safari/537.36"
127.0.0.1 - - [07/Dec/2016:11:04:58 +0100] "GET / HTTP/1.1" 304 0 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.12; rv:49.0) Gecko/20100101 Firefox/49.0"
127.0.0.1 - - [07/Dec/2016:11:04:59 +0100] "GET / HTTP/1.1" 304 0 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.12; rv:49.0) Gecko/20100101 Firefox/49.0"
```

#### Input events

The input events contain mocked JSON events that are ready to be passed to the ingest pipeline as-is. Such events can be helpful in situations in which an input event can't be serialized to a standard log file, e.g. Redis input. A sample file with input events  (e.g. `access-event.json`) looks as following:

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

#### Benchmark configuration

The benchmark execution can be customized to some extent using an optional configuration stored as a YAML file with the name `config.yml`:

```yml
num_docs: 1000
```

The `num_docs` option tells the benchmarks how many events should be sent with the simulation request. If not enough samples are provided, the events will be reused to generate a sufficient number of them. If not present it defaults to `1000`.


## Running a pipeline benchmark

Once the configurations are defined as described in the previous section, you are ready to run pipeline benchmarks for a package's data streams.

First you must deploy the Elasticsearch instance. This corresponds to step 1 as described in the [_Conceptual process_](#Conceptual-process) section.

```
elastic-package stack up -d --services=elasticsearch
```

For a complete listing of options available for this command, run `elastic-package stack up -h` or `elastic-package help stack up`.

Next, you must invoke the pipeline benchmark runner. This corresponds to steps 2 through 4 as described in the [_Conceptual process_](#Conceptual-process) section.

If you want to run pipeline benchmarks for **all data streams** in a package, navigate to the package's root folder (or any sub-folder under it) and run the following command.

```
elastic-package benchmark pipeline

--- Benchmark results for package: windows-1 - START ---
╭───────────────────────────────╮
│ parameters                    │
├──────────────────┬────────────┤
│ package          │    windows │
│ data_stream      │ powershell │
│ source doc count │          6 │
│ doc count        │       1000 │
╰──────────────────┴────────────╯
╭───────────────────────╮
│ ingest performance    │
├─────────────┬─────────┤
│ ingest time │   0.23s │
│ eps         │ 4291.85 │
╰─────────────┴─────────╯
╭───────────────────────────────────╮
│ processors by total time          │
├──────────────────────────┬────────┤
│ kv @ default.yml:4       │ 12.02% │
│ script @ default.yml:240 │  7.73% │
│ kv @ default.yml:13      │  6.87% │
│ set @ default.yml:44     │  6.01% │
│ script @ default.yml:318 │  5.58% │
│ date @ default.yml:34    │  3.43% │
│ script @ default.yml:397 │  2.15% │
│ remove @ default.yml:425 │  2.15% │
│ set @ default.yml:102    │  1.72% │
│ set @ default.yml:108    │  1.29% │
╰──────────────────────────┴────────╯
╭─────────────────────────────────────╮
│ processors by average time per doc  │
├──────────────────────────┬──────────┤
│ kv @ default.yml:4       │ 56.112µs │
│ script @ default.yml:240 │ 36.072µs │
│ kv @ default.yml:13      │ 31.936µs │
│ script @ default.yml:397 │  29.94µs │
│ set @ default.yml:44     │     14µs │
│ script @ default.yml:318 │     13µs │
│ date @ default.yml:34    │ 11.976µs │
│ set @ default.yml:102    │  8.016µs │
│ append @ default.yml:114 │  6.012µs │
│ set @ default.yml:108    │  6.012µs │
╰──────────────────────────┴──────────╯

--- Benchmark results for package: windows-1 - END   ---
Done

```

If you want to run pipeline benchmarks for **specific data streams** in a package, navigate to the package's root folder (or any sub-folder under it) and run the following command.

```
elastic-package benchmark pipeline --data-streams <data stream 1>[,<data stream 2>,...]
```

By default, if the benchmark configuration is not present, it will run using any samples found in the data stream. You can disable this behavior disabling the `--use-test-samples` flag.

```
elastic-package benchmark pipeline -v --use-test-samples=false
```

Finally, when you are done running all benchmarks, bring down the Elastic Stack. This corresponds to step 4 as described in the [_Conceptual process_](#Conceptual-process) section.

```
elastic-package stack down
```
