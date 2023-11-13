# HOWTO: Writing rally benchmarks for a package

## Introduction
Elastic Packages are comprised of data streams. A rally benchmark runs `esrally` track with a corpus of data into an Elasticsearch data stream, and reports rally stats as well as retrieving performance metrics from the Elasticsearch nodes.

## Conceptual process

Conceptually, running a rally benchmark involves the following steps:

1. Deploy the Elastic Stack, including Elasticsearch, Kibana, and the Elastic Agent(s). This step takes time so it should typically be done once as a pre-requisite to running a system benchmark scenario.
1. Install a package that configures its assets for every data stream in the package.
1. Metrics collections from the cluster starts. (**TODO**: record metrics from all Elastic Agents involved using the `system` integration.)
1. Generate data (it uses the [corpus-generator-tool](https://github.com/elastic/elastic-integration-corpus-generator-tool))
1. Run an `esrally` track with the corpus of generated data. `esrally` must be installed on the system where the `elastic-package` is run and available in the `PATH`. 
1. Wait for the `esrally` track to be executed.
1. Metrics collection ends and a summary report is created.
1. Send the collected metrics to the ES Metricstore if set.
1. Delete test artifacts.
1. Optionally reindex all ingested data into the ES Metricstore for further analysis.
1. **TODO**: Optionally compare results against another benchmark run.

### Benchmark scenario definition

We must define at least one configuration for the package that we
want to benchmark. There can be multiple scenarios defined for the same package.

```
<package root>/
  _dev/
    benchmark/
      rally/
        <scenario>.yml
```

The `<scenario>.yml` files allow you to define various settings for the benchmark scenario
along with values for package and data stream-level variables. These are the available configuration options for system benchmarks.

| Option                          | Type       | Required | Description                                                                                                                                                           |
|---------------------------------|------------|---|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| package                         | string     | | The name of the package. If omitted will pick the current package, this is to allow for future definition of benchmarks outside of the packages folders.              |
| description                     | string     | | A description for the scenario.                                                                                                                                       |
| version                         | string     | | The version of the package to benchmark. If omitted will pick the current version of the package.                                                                     |
| data_stream.name                | string     | yes | The data stream to benchmark.                                                                                                                                         |
| warmup_time_period              | duration   |  | Warmup time period. All data prior to this period will be ignored in the benchmark results.                                                                           |
| corpora.generator.total_events  | uint64     |  | Number of total events to generate. Example: `20000`                                                                                                                  |
| corpora.generator.template.raw  | string     |  | Raw template for the corpus generator.                                                                                                                                |
| corpora.generator.template.path | string     |  | Path to the template for the corpus generator. If a `path` is defined, it will override any `raw` template definition.                                                |
| corpora.generator.template.type | string     |  | Type of the template for the corpus generator. Default `placeholder`.                                                                                                 |
| corpora.generator.config.raw    | dictionary |  | Raw config for the corpus generator.                                                                                                                                  |
| corpora.generator.config.path   | string     |  | Path to the config for the corpus generator. If a `path` is defined, it will override any `raw` config definition.                                                    |
| corpora.generator.fields.raw    | dictionary |  | Raw fields for the corpus generator.                                                                                                                                  |
| corpora.generator.fields.path   | string     |  | Path to the fields for the corpus generator. If a `path` is defined, it will override any `raw` fields definition.                                                    |

Example:

`logs-benchmark.yml`
```yaml
---
---
description: Benchmark 20000 events ingested
data_stream:
   name: testds
corpora:
   generator:
      total_events: 900000
      template:
         type: gotext
         path: ./logs-benchmark/template.ndjson
      config:
         path: ./logs-benchmark/config.yml
      fields:
         path: ./logs-benchmark/fields.yml
```

There is no need to define an `input` and `vars` for the package, 
since we don't create any agent policy, and we don't enroll any agent.

## Running a rally benchmark

Once the configuration is defined as described in the previous section, you are ready to run rally benchmarks for a package.

First you must deploy the Elastic Stack. 

```
elastic-package stack up -d
```

For a complete listing of options available for this command, run `elastic-package stack up -h` or `elastic-package help stack up`.

Next, you must invoke the system benchmark runner.

```
elastic-package benchmark rally --benchmark logs-benchmark -v
# ... debug output
--- Benchmark results for package: rally_benchmarks - START ---
╭──────────────────────────────────────────────────────────────────────────────────────────────────╮
│ info                                                                                             │
├────────────────────────┬─────────────────────────────────────────────────────────────────────────┤
│ benchmark              │                                                          logs-benchmark │
│ description            │                                         Benchmark 20000 events ingested │
│ run ID                 │                                    cb62ba92-14b9-4562-98ce-9251e0936a5e │
│ package                │                                                        rally_benchmarks │
│ start ts (s)           │                                                              1698892087 │
│ end ts (s)             │                                                              1698892134 │
│ duration               │                                                                     47s │
│ generated corpora file │ /Users/andreaspacca/.elastic-package/tmp/rally_corpus/corpus-3633691003 │
╰────────────────────────┴─────────────────────────────────────────────────────────────────────────╯
╭────────────────────────────────────────────────────────────────────╮
│ parameters                                                         │
├─────────────────────────────────┬──────────────────────────────────┤
│ package version                 │                      999.999.999 │
│ data_stream.name                │                           testds │
│ warmup time period              │                              10s │
│ corpora.generator.total_events  │                           900000 │
│ corpora.generator.template.path │ ./logs-benchmark/template.ndjson │
│ corpora.generator.template.raw  │                                  │
│ corpora.generator.template.type │                           gotext │
│ corpora.generator.config.path   │      ./logs-benchmark/config.yml │
│ corpora.generator.config.raw    │                            map[] │
│ corpora.generator.fields.path   │      ./logs-benchmark/fields.yml │
│ corpora.generator.fields.raw    │                            map[] │
╰─────────────────────────────────┴──────────────────────────────────╯
╭───────────────────────╮
│ cluster info          │
├───────┬───────────────┤
│ name  │ elasticsearch │
│ nodes │             1 │
╰───────┴───────────────╯
╭──────────────────────────────────────────────────────────────╮
│ data stream stats                                            │
├────────────────────────────┬─────────────────────────────────┤
│ data stream                │ logs-rally_benchmarks.testds-ep │
│ approx total docs ingested │                          900000 │
│ backing indices            │                               1 │
│ store size bytes           │                       440617124 │
│ maximum ts (ms)            │                   1698892076196 │
╰────────────────────────────┴─────────────────────────────────╯
╭───────────────────────────────────────╮
│ disk usage for index .ds-logs-rally_b │
│ enchmarks.testds-ep-2023.11.01-000001 │
│ (for all fields)                      │
├──────────────────────────────┬────────┤
│ total                        │ 262 MB │
│ inverted_index.total         │  84 MB │
│ inverted_index.stored_fields │  96 MB │
│ inverted_index.doc_values    │  72 MB │
│ inverted_index.points        │ 9.7 MB │
│ inverted_index.norms         │    0 B │
│ inverted_index.term_vectors  │    0 B │
│ inverted_index.knn_vectors   │    0 B │
╰──────────────────────────────┴────────╯
╭────────────────────────────────────────────────────────────────────────────────────────────╮
│ pipeline logs-rally_benchmarks.testds-999.999.999 stats in node 7AYCd2EXQaCSOf-0fKxFBg     │
├────────────────────────────────────────────────┬───────────────────────────────────────────┤
│ Totals                                         │ Count: 900000 | Failed: 0 | Time: 38.993s │
│ grok ()                                        │ Count: 900000 | Failed: 0 | Time: 36.646s │
│ user_agent ()                                  │  Count: 900000 | Failed: 0 | Time: 1.368s │
│ pipeline (logs-rally_benchmarks.testds@custom) │   Count: 900000 | Failed: 0 | Time: 102ms │
╰────────────────────────────────────────────────┴───────────────────────────────────────────╯
╭────────────────────────────────────────────────────────────────────────────────────────────╮
│ rally stats                                                                                │
├────────────────────────────────────────────────────────────────┬───────────────────────────┤
│ Cumulative indexing time of primary shards                     │    3.3734666666666664 min │
│ Min cumulative indexing time across primary shards             │                     0 min │
│ Median cumulative indexing time across primary shards          │  0.046950000000000006 min │
│ Max cumulative indexing time across primary shards             │    1.7421333333333335 min │
│ Cumulative indexing throttle time of primary shards            │                     0 min │
│ Min cumulative indexing throttle time across primary shards    │                     0 min │
│ Median cumulative indexing throttle time across primary shards │                   0.0 min │
│ Max cumulative indexing throttle time across primary shards    │                     0 min │
│ Cumulative merge time of primary shards                        │    0.8019666666666667 min │
│ Cumulative merge count of primary shards                       │                       449 │
│ Min cumulative merge time across primary shards                │                     0 min │
│ Median cumulative merge time across primary shards             │              0.009525 min │
│ Max cumulative merge time across primary shards                │                0.5432 min │
│ Cumulative merge throttle time of primary shards               │   0.21998333333333334 min │
│ Min cumulative merge throttle time across primary shards       │                     0 min │
│ Median cumulative merge throttle time across primary shards    │                   0.0 min │
│ Max cumulative merge throttle time across primary shards       │   0.21998333333333334 min │
│ Cumulative refresh time of primary shards                      │   0.41008333333333336 min │
│ Cumulative refresh count of primary shards                     │                     14095 │
│ Min cumulative refresh time across primary shards              │                     0 min │
│ Median cumulative refresh time across primary shards           │  0.011966666666666665 min │
│ Max cumulative refresh time across primary shards              │    0.2056333333333333 min │
│ Cumulative flush time of primary shards                        │               8.79825 min │
│ Cumulative flush count of primary shards                       │                     13859 │
│ Min cumulative flush time across primary shards                │ 6.666666666666667e-05 min │
│ Median cumulative flush time across primary shards             │   0.45098333333333335 min │
│ Max cumulative flush time across primary shards                │    0.6979833333333333 min │
│ Total Young Gen GC time                                        │                   0.646 s │
│ Total Young Gen GC count                                       │                       147 │
│ Total Old Gen GC time                                          │                       0 s │
│ Total Old Gen GC count                                         │                         0 │
│ Store size                                                     │    0.23110682144761086 GB │
│ Translog size                                                  │     0.5021391455084085 GB │
│ Heap used for segments                                         │                      0 MB │
│ Heap used for doc values                                       │                      0 MB │
│ Heap used for terms                                            │                      0 MB │
│ Heap used for norms                                            │                      0 MB │
│ Heap used for points                                           │                      0 MB │
│ Heap used for stored fields                                    │                      0 MB │
│ Segment count                                                  │                       325 │
│ Total Ingest Pipeline count                                    │                    900063 │
│ Total Ingest Pipeline time                                     │                  51.994 s │
│ Total Ingest Pipeline failed                                   │                         0 │
│ Min Throughput                                                 │           23987.18 docs/s │
│ Mean Throughput                                                │           46511.27 docs/s │
│ Median Throughput                                              │           49360.50 docs/s │
│ Max Throughput                                                 │           51832.58 docs/s │
│ 50th percentile latency                                        │      645.8979995000007 ms │
│ 90th percentile latency                                        │       896.532670700001 ms │
│ 99th percentile latency                                        │     1050.0004142499988 ms │
│ 100th percentile latency                                       │      1064.915250000002 ms │
│ 50th percentile service time                                   │      645.8979995000007 ms │
│ 90th percentile service time                                   │       896.532670700001 ms │
│ 99th percentile service time                                   │     1050.0004142499988 ms │
│ 100th percentile service time                                  │      1064.915250000002 ms │
│ error rate                                                     │                    0.00 % │
╰────────────────────────────────────────────────────────────────┴───────────────────────────╯

--- Benchmark results for package: rally_benchmarks - END   ---
Done
```

Finally, when you are done running the benchmark, bring down the Elastic Stack. 

```
elastic-package stack down
```

## Setting up an external metricstore

A metricstore can be set up to send metrics collected during the benchmark execution.

An external metricstore might be useful for:

- Store monitoring data of the benchmark scenario for all its execution time.
- Analyse the data generated during a benchmark. This is possible when using the `reindex-to-metricstore` flag.
- **TODO**: Store benchmark results for various benchmark runs permanently for later comparison.

In order to initialize it, you need to set up the following environment variables:

```bash
export ELASTIC_PACKAGE_ESMETRICSTORE_HOST=https://127.0.0.1:9200
export ELASTIC_PACKAGE_ESMETRICSTORE_USERNAME=elastic
export ELASTIC_PACKAGE_ESMETRICSTORE_PASSWORD=changeme
export ELASTIC_PACKAGE_ESMETRICSTORE_CA_CERT="$HOME/.elastic-package/profiles/default/certs/ca-cert.pem"
```

The only one that is optional is `ELASTIC_PACKAGE_ESMETRICSTORE_CA_CERT`.

When these are detected, metrics will be automatically collected every second and sent to a new index called `bench-metrics-{dataset}-{testRunID}"`.

The collected metrics include the following node stats: `nodes.*.breakers`, `nodes.*.indices`, `nodes.*.jvm.mem`, `nodes.*.jvm.gc`, `nodes.*.jvm.buffer_pools`, `nodes.*.os.mem`, `nodes.*.process.cpu`, `nodes.*.thread_pool`, and `nodes.*.transport`.

Ingest pipelines metrics are only collected at the end since its own collection would affect the benchmark results.

You can see a sample collected metric [here](./sample_metric.json)

Additionally, if the `reindex-to-metricstore` flag is used, the data generated during the benchmark will be sent to the metricstore into an index called `bench-reindex-{datastream}-{testRunID}` for further analysis. The events will be enriched with metadata related to the benchmark run.

## Persisting rally tracks and dry-run

If the `rally-track-output-dir` flag is used, the track and the corpus generated during the benchmark will be saved in the directory passed as value of the flag.
Additionally, if the `dry-run` flag is used, the command will exits before running `esrally`.
If both the flags above are used at the same time, the command will just generate the track and corpus and save them, without running any benchmark.
If the `dry-run` flag only is used, the command will just wipe the data stream returning no report.

## Replaying a persisted rally track
In the directory of the `rally-track-output-dir` flag two files are saved:
1. The rally track: `track-%data_stream.type%-%data_stream.dataset%-%data_stream.namespace%.json`
2. The track corpus: `corpus-%unix_timestamp%`

Both files are required to replay the rally benchmark. The first file references the second in its content.
The command to run for replaying the track is the following:
```shell
esrally --target-hosts='{"defauelt":["%es_cluster_host:es_cluster_port%"]}' --track-path=%path/to/saved-track-json% --client-options='{"default":{"basic_auth_user":"%es_user%","basic_auth_password":"%es_user%","use_ssl":true,"verify_certs":false}}' --pipeline=benchmark-only 
```

Please refer to [esrally CLI reference](https://esrally.readthedocs.io/en/stable/command_line_reference.html) for more details.

