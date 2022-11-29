# HOWTO: Writing system tests for a package

## Introduction
Elastic Packages are comprised of data streams. A system test exercises the end-to-end flow of data for a package's data stream — from ingesting data from the package's integration service all the way to indexing it into an Elasticsearch data stream.

## Conceptual process

Conceptually, running a system test involves the following steps:

1. Deploy the Elastic Stack, including Elasticsearch, Kibana, and the Elastic Agent. This step takes time so it should typically be done once as a pre-requisite to running system tests on multiple data streams.
1. Enroll the Elastic Agent with Fleet (running in the Kibana instance). This step also can be done once, as a pre-requisite.
1. Depending on the Elastic Package whose data stream is being tested, deploy an instance of the package's integration service.
1. Create a test policy that configures a single data stream for a single package.
1. Assign the test policy to the enrolled Agent.
1. Wait a reasonable amount of time for the Agent to collect data from the
   integration service and index it into the correct Elasticsearch data stream.
1. Query the first 500 documents based on `@timestamp` for validation.
1. Validate mappings are defined for the fields contained in the indexed documents.
1. Validate that the JSON data types contained `_source` are compatible with
   mappings declared for the field.
1. Delete test artifacts and tear down the instance of the package's integration service.
1. Once all desired data streams have been system tested, tear down the Elastic Stack.

## Limitations

At the moment system tests have limitations. The salient ones are:
* There isn't a way to do assert that the indexed data matches data from a file (e.g. golden file testing).

## Defining a system test

Packages have a specific folder structure (only relevant parts shown).

```
<package root>/
  data_stream/
    <data stream>/
      manifest.yml
  manifest.yml
```

To define a system test we must define configuration on at least one level: a package or a data stream's one.

First, we must define the configuration for deploying a package's integration service. We can define it on either the package level:

```
<package root>/
  _dev/
    deploy/
      <service deployer>/
        <service deployer files>
```

or the data stream's level:

```
<package root>/
  data_stream/
    <data stream>/
      _dev/
        deploy/
          <service deployer>/
            <service deployer files>
```

`<service deployer>` - a name of the supported service deployer:
* `docker` - Docker Compose
* `agent` - Custom `elastic-agent` with Docker Compose
* `k8s` - Kubernetes
* `tf` - Terraform

### Docker Compose service deployer

When using the Docker Compose service deployer, the `<service deployer files>` must include a `docker-compose.yml` file.
The `docker-compose.yml` file defines the integration service(s) for the package. If your package has a logs data stream,
the log files from your package's integration service must be written to a volume. For example, the `apache` package has
the following definition in it's integration service's `docker-compose.yml` file.

```
version: '2.3'
services:
  apache:
    # Other properties such as build, ports, etc.
    volumes:
      - ${SERVICE_LOGS_DIR}:/usr/local/apache2/logs
```

Here, `SERVICE_LOGS_DIR` is a special keyword. It is something that we will need later.

`elastic-package` will remove orphan volumes associated to the started services
when they are stopped. Docker compose may not be able to find volumes defined in
the Dockerfile for this cleanup. In these cases, override the volume definition.

For example docker images for MySQL include a volume for the data directory
`/var/lib/mysql`. In order for `elastic-package` to clean up these volumes after
tests are executed, a volume can be added to the `docker-compose.yml`:

```
version: '2.3'
services:
  mysql:
    # Other properties such as build, ports, etc.
    volumes:
      # Other volumes.
      - mysqldata:/var/lib/mysql

volumes:
  mysqldata:
```

### Agent service deployer

When using the Agent service deployer, the `elastic-agent` provided by the stack
will not be used. An agent will be deployed as a Docker compose service named `docker-custom-agent`
which base configuration is provided [here](../../internal/install/_static/docker-custom-agent-base.yml).
This configuration will be merged with the one provided in the `custom-agent.yml` file.
This is useful if you need different capabilities than the provided by the
`elastic-agent` used by the `elastic-package stack` command.

`custom-agent.yml`
```
version: '2.3'
services:
  docker-custom-agent:
    pid: host
    cap_add:
      - AUDIT_CONTROL
      - AUDIT_READ
    user: root
```

This will result in an agent configuration such as:

```
version: '2.3'
services:
  docker-custom-agent:
    hostname: docker-custom-agent
    image: "docker.elastic.co/beats/elastic-agent-complete:8.2.0"
    pid: host
    cap_add:
      - AUDIT_CONTROL
      - AUDIT_READ
    user: root
    healthcheck:
      test: "elastic-agent status"
      retries: 180
      interval: 1s
    environment:
      FLEET_ENROLL: "1"
      FLEET_INSECURE: "1"
      FLEET_URL: "http://fleet-server:8220"
```

And in the test config:

```
data_stream:
  vars:
  # ...
```

#### Agent service deployer with multiple services

Multiple services may need to be created to meet specific requirements. For example, a custom elastic-agent having certain libraries installed is required to connect with the service that is intended for testing.

`Dockerfile` having the image build instructions, `custom-agent.yml` having docker-compose definition for configuring the custom-agent can be kept in the below location

```
<package root>/
  _dev/
    deploy/
      <service deployer>/
        <service deployer files>
```

or the data stream's level:

```
<package root>/
  data_stream/
    <data stream>/
      _dev/
        deploy/
          <service deployer>/
            <service deployer files>
```

An example for `Dockerfile` is as below
```
FROM docker.elastic.co/elastic-agent/elastic-agent-complete:8.4.0
USER root
RUN apt-get update && apt-get -y install \
    libaio1 \
    wget \
    unzip
```
An example for `custom-agent.yml` in multi-service setup is as below
```
version: '2.3'
services:
  docker-custom-agent:
    build:
      context: .
      dockerfile: Dockerfile
    image: elastic-agent-oracle-client-1
    depends_on:
        oracle:
          condition: service_healthy
    healthcheck:
      test: ["CMD", "bash", "-c", "echo 'select sysdate from dual;' | ORACLE_HOME=/opt/oracle/instantclient_21_4 /opt/oracle/instantclient_21_4/sqlplus -s <user>/<password>@oracle:1521/ORCLCDB.localdomain as sysdba"]
      interval: 120s
      timeout: 300s
      retries: 300

  oracle:
    image: docker.elastic.co/observability-ci/database-enterprise:12.2.0.1
    container_name: oracle
    ports:
      - 1521:1521
      - 5500:5500
    healthcheck:
      test: ["CMD", "bash", "-c", "echo 'select sysdate from dual;' | ORACLE_HOME=/u01/app/oracle/product/12.2.0/dbhome_1/ /u01/app/oracle/product/12.2.0/dbhome_1/bin/sqlplus -s <user>/<password>@oracle:1521/ORCLCDB.localdomain as sysdba"]
      interval: 120s
      timeout: 300s
      retries: 300
```


### Terraform service deployer

When using the Terraform service deployer, the `<service deployer files>` must include at least one `*.tf` file.
The `*.tf` files define the infrastructure using the Terraform syntax. The terraform based service can be handy to boot up
resources using selected cloud provider and use them for testing (e.g. observe and collect metrics).

Sample `main.tf` definition:

```
variable "TEST_RUN_ID" {
  default = "detached"
}

provider "aws" {}

resource "aws_instance" "i" {
  ami           = data.aws_ami.latest-amzn.id
  monitoring = true
  instance_type = "t1.micro"
  tags = {
    Name = "elastic-package-test-${var.TEST_RUN_ID}"
  }
}

data "aws_ami" "latest-amzn" {
  most_recent = true
  owners = [ "amazon" ] # AWS
  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*"]
  }
}
```

Notice the use of the `TEST_RUN_ID` variable. It contains a unique ID, which can help differentiate resources created in potential concurrent test runs.

#### Environment variables

To use environment variables within the Terraform service deployer a `env.yml` file is required.

The file should be structured like this:

```yaml
version: '2.3'
services:
  terraform:
    environment:
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
```

It's purpose is to inject environment variables in the Terraform service deployer environment.

To specify a default use this syntax: `AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID:-default}`, replacing `default` with the desired default value.

**NOTE**: Terraform requires to prefix variables using the environment variables form with `TF_VAR_`. These variables are not available in test case definitions because they are [not injected](https://github.com/elastic/elastic-package/blob/f5312b6022e3527684e591f99e73992a73baafcf/internal/testrunner/runners/system/servicedeployer/terraform_env.go#L43) in the test environment.

#### Cloud Provider CI support

Terraform is often used to interact with Cloud Providers. This require Cloud Provider credentials.

Injecting credentials can be achieved with functions from the [`apm-pipeline-library`](https://github.com/elastic/apm-pipeline-library/tree/main/vars) Jenkins library. For example look for `withAzureCredentials`, `withAWSEnv` or `withGCPEnv`.

#### Tagging/labelling created Cloud Provider resources

Leveraging Terraform to create cloud resources is useful but risks creating leftover resources that are difficult to remove.

There are some specific environment variables that should be leveraged to overcome this issue; these variables are already injected to be used by Terraform (through `TF_VAR_`):
- `TF_VAR_TEST_RUN_ID`: a unique identifier for the test run, allows to distinguish each run
- `BRANCH_NAME_LOWER_CASE`: the branch name or PR number the CI run is linked to
- `BUILD_ID`: incremental number providing the current CI run number
- `CREATED_DATE`: the creation date in epoch time, milliseconds, when the resource was created
- `ENVIRONMENT`: what environment created the resource (`ci`)
- `REPO`: the GitHub repository name (`elastic-package`)

### Kubernetes service deployer

The Kubernetes service deployer requires the `_dev/deploy/k8s` directory to be present. It can include additional `*.yaml` files to deploy
custom applications in the Kubernetes cluster (e.g. Nginx deployment). If no resource definitions (`*.yaml` files ) are needed,
the `_dev/deploy/k8s` directory must contain an `.empty` file (to preserve the `k8s` directory under version control).

The Kubernetes service deployer needs [kind](https://kind.sigs.k8s.io/) to be installed and the cluster to be up and running:

```bash
wget -qO-  https://raw.githubusercontent.com/elastic/elastic-package/main/scripts/kind-config.yaml | kind create cluster --config -
```

Before executing system tests, the service deployer applies once the deployment of the Elastic Agent to the cluster and links
the kind cluster with the Elastic stack network - applications running in the kind cluster can reach Elasticsearch and Kibana instances.
To shorten the total test execution time the Elastic Agent's deployment is not deleted after tests, but it can be reused.

See how to execute system tests for the Kubernetes integration (`pod` data stream):

```bash
elastic-package stack up -d -v # start the Elastic stack
wget -qO-  https://raw.githubusercontent.com/elastic/elastic-package/main/scripts/kind-config.yaml | kind create cluster --config -
elastic-package test system --data-streams pod -v # start system tests for the "pod" data stream
```

### Test case definition

Next, we must define at least one configuration for each data stream that we
want to system test. There can be multiple test cases defined for the same data
stream.

_Hint: if you plan to define only one test case, you can consider the filename
`test-default-config.yml`._

```
<package root>/
  data_stream/
    <data stream>/
      _dev/
        test/
          system/
            test-<test_name>-config.yml
```

The `test-<test_name>-config.yml` file allows you to define values for package
and data stream-level variables. These are the available configuration options
for system tests.

| Option | Type | Required | Description |
|---|---|---|---|
| data_stream.vars | dictionary |  | Data stream level variables to set (i.e. declared in `package_root/data_stream/$data_stream/manifest.yml`). If not specified the defaults from the manifest are used. |
| input | string | yes | Input type to test (e.g. logfile, httpjson, etc). Defaults to the input used by the first stream in the data stream manifest. |
| numeric_keyword_fields | []string |  | List of fields to ignore during validation that are mapped as `keyword` in Elasticsearch, but their JSON data type is a number. |
| policy_template | string |  | Name of policy template associated with the data stream and input. Required when multiple policy templates include the input being tested. |
| service | string |  | Name of a specific Docker service to setup for the test. |
| service_notify_signal | string |  | Signal name to send to 'service' when the test policy has been applied to the Agent. This can be used to trigger the service after the Agent is ready to receive data. |
| skip.link | URL |  | URL linking to an issue about why the test is skipped. |
| skip.reason | string |  | Reason to skip the test. If specified the test will not execute. |
| vars | dictionary |  | Package level variables to set (i.e. declared in `$package_root/manifest.yml`). If not specified the defaults from the manifest are used. |
| wait_for_data_timeout | duration |  | Amount of time to wait for data to be present in Elasticsearch. Defaults to 10m. |

For example, the `apache/access` data stream's `test-access-log-config.yml` is
shown below.

```yaml
vars: ~
input: logfile
data_stream:
  vars:
    paths:
      - "{{{SERVICE_LOGS_DIR}}}/access.log*"
```

The top-level `vars` field corresponds to package-level variables defined in the
`apache` package's `manifest.yml` file. In the above example we don't override
any of these package-level variables, so their default values, as specified in
the `apache` package's `manifest.yml` file are used.

The `data_stream.vars` field corresponds to data stream-level variables for the
current data stream (`apache/access` in the above example). In the above example
we override the `paths` variable. All other variables are populated with their
default values, as specified in the `apache/access` data stream's `manifest.yml`
file.

Notice the use of the `{{{SERVICE_LOGS_DIR}}}` placeholder. This corresponds to
the `${SERVICE_LOGS_DIR}` variable we saw in the `docker-compose.yml` file
earlier. In the above example, the net effect is as if the
`/usr/local/apache2/logs/access.log*` files located inside the Apache
integration service container become available at the same path from Elastic
Agent's perspective.

When a data stream's manifest declares multiple streams with different inputs
you can use the `input` option to select the stream to test. The first stream
whose input type matches the `input` value will be tested. By default, the first
stream declared in the manifest will be tested.

#### Placeholders

The `SERVICE_LOGS_DIR` placeholder is not the only one available for use in a data stream's `test-<test_name>-config.yml` file. The complete list of available placeholders is shown below.

| Placeholder name | Data type | Description |
| --- | --- | --- |
| `Hostname`| string | Addressable host name of the integration service. |
| `Ports` | []int | Array of addressable ports the integration service is listening on. |
| `Port` | int | Alias for `Ports[0]`. Provided as a convenience. |
| `Logs.Folder.Agent` | string | Path to integration service's logs folder, as addressable by the Agent. |
| `SERVICE_LOGS_DIR` | string | Alias for `Logs.Folder.Agent`. Provided as a convenience. |

Placeholders used in the `test-<test_name>-config.yml` must be enclosed in `{{{` and `}}}` delimiters, per Handlebars syntax.


**NOTE**: Terraform variables in the form of environment variables (prefixed with `TF_VAR_`) are not injected and cannot be used as placeholder (their value will always be empty).

## Running a system test

Once the two levels of configurations are defined as described in the previous section, you are ready to run system tests for a package's data streams.

First you must deploy the Elastic Stack. This corresponds to steps 1 and 2 as described in the [_Conceptual process_](#Conceptual-process) section.

```
elastic-package stack up -d
```

For a complete listing of options available for this command, run `elastic-package stack up -h` or `elastic-package help stack up`.

Next, you must set environment variables needed for further `elastic-package` commands.

```
$(elastic-package stack shellinit)
```

Next, you must invoke the system tests runner. This corresponds to steps 3 through 7 as described in the [_Conceptual process_](#Conceptual-process) section.

If you want to run system tests for **all data streams** in a package, navigate to the package's root folder (or any sub-folder under it) and run the following command.

```
elastic-package test system
```

If you want to run system tests for **specific data streams** in a package, navigate to the package's root folder (or any sub-folder under it) and run the following command.

```
elastic-package test system --data-streams <data stream 1>[,<data stream 2>,...]
```

Finally, when you are done running all system tests, bring down the Elastic Stack. This corresponds to step 8 as described in the [_Conceptual process_](#Conceptual_process) section.

```
elastic-package stack down
```

### Generating sample events

As the system tests exercise an integration end-to-end from running the integration's service all the way
to indexing generated data from the integration's data streams into Elasticsearch, it is possible to generate
`sample_event.json` files for each of the integration's data streams while running these tests.

```
elastic-package test system --generate
```

## Continuous Integration

`elastic-package` runs a set of system tests on some [dummy packages](https://github.com/elastic/elastic-package/tree/main/test/packages) to ensure it's functionalities work as expected. This allows to test changes affecting package testing within `elastic-package` before merging and releasing the changes.

Tests use set of environment variables that are set at the beginning of the `Jenkinsfile`.

The exposed environment variables are passed to the test runners through service deployer specific configuration (refer to the service deployer section for further details).

### Stack version

The tests use the [default version](https://github.com/elastic/elastic-package/blob/main/internal/install/stack_version.go#L9) `elastic-package` provides.

You can override this value by changing it in your PR if needed. To update the default version always create a dedicated PR.
