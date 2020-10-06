# HOWTO: Writing system tests for a package

## Introduction
Elastic Packages are comprised of data streams. A system test exercises the end-to-end flow of data for a package's data stream — from ingesting data from the package's integration service all the way to indexing it into an Elasticsearch data stream.

## Process

Conceptually, a system test will perform the following steps:
1. Deploy the Elastic Stack, including a 1-node Elasticsearch cluster, a Kibana instance, and an instance of Elastic Agent.
1. Enroll the Elastic Agent with Fleet (running in the Kibana instance).
1. Depending on the Elastic Package whose data stream is being tested, deploy an instance of the package's integration service.
1. Create a test policy that configures a single data stream for a single package.
1. Assign the test policy to the enrolled Agent.
1. Wait a reasonable amount of time for the Agent to collect data from the integration service and index it into the correct Elasticsearch data stream.
1. Delete test artifacts and tear down deployed resources.

## Limitations

At the moment system tests have limitations. The salient ones are:
* They can only test package's whose integration services can be deployed via Docker Compose. Eventually they will be able to test package's that can be deployed via other means, e.g. a Terraform configuration.
* They can only check for the _existence_ of data in the correct Elasticsearch data stream. Eventually they will be able to test the shape and contents of the indexed data as well.

## Defining a system test

Package's have a folder structure like so (only relevant parts shown):

```
<package root>/
  data_stream/
    <data stream>/
      manifest.yml
  manifest.yml
```

To define a system test we must define configuration at two levels: the package level and each dataset's level.

### Package-level configuration

First, we must define the configuration for deploying a package's integration service. As mentioned in the [_Limitations_](#Limitations) section above, only package's whose integration services can be deployed via Docker Compose are supported at the moment. Configuration for such package's looks like so:

```
<package root>/
  _dev/
    deploy/
      docker/
        docker-compose.yml
```

The `docker-compose.yml` file defines the integration service(s) for the package. If your package has a logs data stream, the log files from your package's integration service must be written to a volume. For example, the `apache` package has the following definition in it's integration service's `docker-compose.yml` file:

```
version: '2.3'
services:
  apache:
    # Other properties such as build, ports, etc.
    volumes:
      - ${SERVICE_LOGS_DIR}:/usr/local/apache2/logs
```

Here, `SERVICE_LOGS_DIR` is a special keyword. It is something that we will need later.

### Dataset-level configuration

Next, we must define configuration for each data stream that we want to system test, like so:

```
<package root>/
  data_stream/
    <data stream>/
      _dev/
        test/
          system/
            config.yml
```

The `config.yml` file allows you define values for package and data stream-level variables. For example, the `apache/access` data stream's `config.yml` looks like this:

```
vars: ~
data_stream:
  vars:
    paths:
      - "{{SERVICE_LOGS_DIR}}/access.log*"
```

The top-level `vars` field corresponds to package-level variables defined in the `apache` package's `manifest.yml` file. In the above example we don't override any of these package-level variables, so their default values, as specified in the `apache` package's `manifest.yml` file are used.

The `data_stream.vars` field corresponds to data stream-level variables for the current data stream (`apache/access` in the above example). In the above example we override the `paths` variable. All other variables are populated with their default values, as specified in the `apache/access` data stream's `manifest.yml` file.

Notice the use of the `{{SERVICE_LOGS_DIR}}` placeholder. This corresponds to the `${SERVICE_LOGS_DIR}` variable we saw in the `docker-compose.yml` file earlier. In the above example, the net effect is as if the `/usr/local/apache2/logs/access.log*` files located inside the Apache integration service container become available at the same path from Elastic Agent's perspective.

#### Placeholders

The `SERVICE_LOGS_DIR` placeholder is not the only one available for use in a data stream's `config.yml` file. The complete list of available placeholder is shown below.

| Placeholder name | Data type | Description |
| --- | --- | --- |
| `Hostname`| string | Addressable host name of the integration service. |
| `Ports` | []int | Array of addressable ports the integration service is listening on. |
| `Port` | int | Alias for `Ports[0]`. Provided as a convenience. |
| `Logs.Folder.Agent` | string | Path to integration service's logs folder, as addressable by the Agent. |
| `SERVICE_LOGS_DIR` | string | Alias for `Logs.Folder.Agent`. Provided as a convenience. |


## Running a system test



