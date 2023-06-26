# HOWTO: Writing system benchmarks for a package

## Introduction
Elastic Packages are comprised of data streams. A system benchmark exercises the end-to-end flow of data for a package's data stream — from ingesting data from the package's integration service all the way to indexing it into an Elasticsearch data stream, and retrieves performance metrics from the Elasticsearch nodes.

## Conceptual process

Conceptually, running a system benchmark involves the following steps:

1. Deploy the Elastic Stack, including Elasticsearch, Kibana, and the Elastic Agent(s). This step takes time so it should typically be done once as a pre-requisite to running a system benchmark scenario.
1. Enroll the Elastic Agent(s) with Fleet (running in the Kibana instance). This step also can be done once, as a pre-requisite.
1. Depending on the Elastic Package whose data stream is being tested, deploy an instance of the package's integration service.
1. Create a benchmark policy that configures a single data stream for a single package.
1. Assign the policy to the enrolled Agent(s).
1. Metrics collections from the cluster starts. (**TODO**: record metrics from all Elastic Agents involved using the `system` integration.)
1. **TODO**: Send the collected metrics to the ES Metricstore if set.
1. Generate data if configured (it uses the [corpus-generator-rool](https://github.com/elastic/elastic-integration-corpus-generator-tool))
1. Wait a reasonable amount of time for the Agent to collect data from the
   integration service and index it into the correct Elasticsearch data stream.
   This time can be pre-defined with the `benchmark_time`. In case this setting is not set
   the benchmark will continue until the number of documents is not changed in the data stream.
1. Metrics collection ends and a summary report is created.
1. Delete test artifacts and tear down the instance of the package's integration service.
1. Optionally reindex all ingested data into the ES Metricstore for further analysis.
1. **TODO**: Optionally compare results against another benchmark run.

## Defining a system benchmark scenario

System benchmarks are defined at the package level.

Optionally system benchmarks can define a configuration for deploying a package's integration service. We must define it on the package level:

```
<package root>/
  _dev/
    benchmark/
      system/
        deploy/
          <service deployer>/
            <service deployer files>
```

`<service deployer>` - a name of the supported service deployer:
* `docker` - Docker Compose

**TODO**: support other service deployers

### Docker Compose service deployer

When using the Docker Compose service deployer, the `<service deployer files>` must include a `docker-compose.yml` file.
The `docker-compose.yml` file defines the integration service(s) for the package. If your package has a logs data stream, the log files from your package's integration service must be written to a volume.

`elastic-package` will remove orphan volumes associated to the started services
when they are stopped. Docker compose may not be able to find volumes defined in
the Dockerfile for this cleanup. In these cases, override the volume definition.

### Benchmark scenario definition

Next, we must define at least one configuration for the package that we
want to benchmark. There can be multiple scenarios defined for the same package.

```
<package root>/
  _dev/
    benchmark/
      system/
        <scenario>.yml
```

The `<scenario>.yml` files allow you to define various settings for the benchmark scenario
along with values for package and data stream-level variables. These are the available configuration options for system benchmarks.

| Option | Type | Required | Description |
|---|---|---|---|
| package | string | | The name of the package. If omitted will pick the current package, this is to allow for future definition of benchmarks outside of the packages folders. |
| description | string | | A description for the scenario. |
| version | string | | The version of the package to benchmark. If omitted will pick the current version of the package. |
| input | string | yes | Input type to test (e.g. logfile, httpjson, etc). Defaults to the input used by the first stream in the data stream manifest. |
| vars | dictionary |  | Package level variables to set (i.e. declared in `$package_root/manifest.yml`). If not specified the defaults from the manifest are used. |
| data_stream.name | string | yes | The data stream to benchmark. |
| data_stream.vars | dictionary |  | Data stream level variables to set (i.e. declared in `package_root/data_stream/$data_stream/manifest.yml`). If not specified the defaults from the manifest are used. |
| warmup_time_period | duration |  | Warmup time period. All data prior to this period will be ignored in the benchmark results. |
| benchmark_time_period | duration |  | Amount of time the benchmark needs to run for. If set the benchmark will stop after this period even though more data is still pending to be ingested. |
| wait_for_data_timeout | duration |  | Amount of time to wait for data to be present in Elasticsearch. Defaults to 10m. |
| corpora.generator.size | string |  | String describing the amount of data to generate. Example: `20MiB` |
| corpora.generator.template.raw | string |  | Raw template for the corpus generator. |
| corpora.generator.template.path | string |  | Path to the template for the corpus generator. If a `path` is defined, it will override any `raw` template definition. |
| corpora.generator.template.type | string |  | Type of the template for the corpus generator. Default `placeholder`. |
| corpora.generator.config.raw | dictionary |  | Raw config for the corpus generator. |
| corpora.generator.config.path | string |  | Path to the config for the corpus generator. If a `path` is defined, it will override any `raw` config definition. |
| corpora.generator.fields.raw | dictionary |  | Raw fields for the corpus generator. |
| corpora.generator.fields.path | string |  | Path to the fields for the corpus generator. If a `path` is defined, it will override any `raw` fields definition. |
| corpora.input_service.name | string |  | Name of the input service to use (defined in the `deploy` folder). |
| corpora.input_service.signal | string |  | Signal to send to the input service once the benchmark is ready to start. |

Example:

`logs-benchmark.yml`
```yaml
---
description: Benchmark 100MiB of data ingested
input: filestream
vars: ~
data_stream.name: test
data_stream.vars.paths:
  - "{{SERVICE_LOGS_DIR}}/corpus-*"
warmup_time_period: 10s
corpora.generator.size: 100MiB
corpora.generator.template.path: ./logs-benchmark/template.log
corpora.generator.config.path: ./logs-benchmark/config.yml
corpora.generator.fields.path: ./logs-benchmark/fields.yml
```

The top-level `vars` field corresponds to package-level variables defined in the
package's `manifest.yml` file. In the above example we don't override
any of these package-level variables, so their default values, as specified in
the package's `manifest.yml` file are used.

The `data_stream.vars` field corresponds to data stream-level variables for the
current data stream (`test` in the above example). In the above example
we override the `paths` variable. All other variables are populated with their
default values, as specified in the data stream's `manifest.yml` file.

Notice the use of the `{{SERVICE_LOGS_DIR}}` placeholder. This also corresponds to
the `${SERVICE_LOGS_DIR}` variable that can be used  the `docker-compose.yml` files. 

**The generator puts the generated data inside `{{SERVICE_LOGS_DIR}}` so they are available both
in `docker-compose.yml` and inside the Elastic Agent that is provisioned by the `stack` command.**


#### Placeholders

The `SERVICE_LOGS_DIR` placeholder is not the only one available for use in a data stream's `<scenario>.yml` file. The complete list of available placeholders is shown below.

| Placeholder name | Data type | Description |
| --- | --- | --- |
| `Hostname`| string | Addressable host name of the integration service. |
| `Ports` | []int | Array of addressable ports the integration service is listening on. |
| `Port` | int | Alias for `Ports[0]`. Provided as a convenience. |
| `Logs.Folder.Agent` | string | Path to integration service's logs folder, as addressable by the Agent. |
| `SERVICE_LOGS_DIR` | string | Alias for `Logs.Folder.Agent`. Provided as a convenience. |

Placeholders used in the `<scenario>.yml` must be enclosed in `{{` and `}}` delimiters, per Go syntax.

## Running a system benchmark

Once the configuration is defined as described in the previous section, you are ready to run system benchmarks for a package.

First you must deploy the Elastic Stack. 

```
elastic-package stack up -d
```

For a complete listing of options available for this command, run `elastic-package stack up -h` or `elastic-package help stack up`.

Next, you must set environment variables needed for further `elastic-package` commands.

```
$(elastic-package stack shellinit)
```

Next, you must invoke the system benchmark runner.

```
elastic-package benchmark system --benchmark logs-benchmark -v
# ... debug output
--- Benchmark results for package: system_benchmarks - START ---
╭─────────────────────────────────────────────────────╮
│ info                                                │
├──────────────┬──────────────────────────────────────┤
│ benchmark    │                       logs-benchmark │
│ description  │    Benchmark 100MiB of data ingested │
│ run ID       │ d2960c04-0028-42c9-bafc-35e599563cb1 │
│ package      │                    system_benchmarks │
│ start ts (s) │                           1682320355 │
│ end ts (s)   │                           1682320355 │
│ duration     │                                 2m3s │
╰──────────────┴──────────────────────────────────────╯
╭───────────────────────────────────────────────────────────────────────╮
│ parameters                                                            │
├─────────────────────────────────┬─────────────────────────────────────┤
│ package version                 │                         999.999.999 │
│ input                           │                          filestream │
│ data_stream.name                │                                test │
│ data_stream.vars.paths          │        [/tmp/service_logs/corpus-*] │
│ warmup time period              │                                 10s │
│ benchmark time period           │                                  0s │
│ wait for data timeout           │                                  0s │
│ corpora.generator.size          │                              100MiB │
│ corpora.generator.template.path │       ./logs-benchmark/template.log │
│ corpora.generator.template.raw  │                                     │
│ corpora.generator.template.type │                                     │
│ corpora.generator.config.path   │         ./logs-benchmark/config.yml │
│ corpora.generator.config.raw    │                               map[] │
│ corpora.generator.fields.path   │         ./logs-benchmark/fields.yml │
│ corpora.generator.fields.raw    │                               map[] │
╰─────────────────────────────────┴─────────────────────────────────────╯
╭───────────────────────╮
│ cluster info          │
├───────┬───────────────┤
│ name  │ elasticsearch │
│ nodes │             1 │
╰───────┴───────────────╯
╭─────────────────────────────────────────────────────────────╮
│ data stream stats                                           │
├────────────────────────────┬────────────────────────────────┤
│ data stream                │ logs-system_benchmarks.test-ep │
│ approx total docs ingested │                         410127 │
│ backing indices            │                              1 │
│ store size bytes           │                      136310570 │
│ maximum ts (ms)            │                  1682320467448 │
╰────────────────────────────┴────────────────────────────────╯
╭───────────────────────────────────────╮
│ disk usage for index .ds-logs-system_ │
│ benchmarks.test-ep-2023.04.22-000001  │
│ (for all fields)                      │
├──────────────────────────────┬────────┤
│ total                        │ 99.8mb │
│ inverted_index.total         │ 31.3mb │
│ inverted_index.stored_fields │ 35.5mb │
│ inverted_index.doc_values    │   30mb │
│ inverted_index.points        │  2.8mb │
│ inverted_index.norms         │     0b │
│ inverted_index.term_vectors  │     0b │
│ inverted_index.knn_vectors   │     0b │
╰──────────────────────────────┴────────╯
╭───────────────────────────────────────────────────────────────────────────────────────────╮
│ pipeline logs-system_benchmarks.test-999.999.999 stats in node Qa9ujRVfQuWhqEESdt6xnw     │
├───────────────────────────────────────────────┬───────────────────────────────────────────┤
│ grok ()                                       │ Count: 407819 | Failed: 0 | Time: 16.615s │
│ user_agent ()                                 │   Count: 407819 | Failed: 0 | Time: 768ms │
│ pipeline (logs-system_benchmarks.test@custom) │    Count: 407819 | Failed: 0 | Time: 59ms │
╰───────────────────────────────────────────────┴───────────────────────────────────────────╯

--- Benchmark results for package: system_benchmarks - END   ---
Done
```

Finally, when you are done running the benchmark, bring down the Elastic Stack. 

```
elastic-package stack down
```

