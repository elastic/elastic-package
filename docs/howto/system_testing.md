# HOWTO: Writing system tests for a package

## Introduction
Elastic Packages are comprised of data streams. A system test exercises the end-to-end flow of data for a package's data stream — from ingesting data from the package's integration service all the way to indexing it into an Elasticsearch data stream.

## Conceptual process

Conceptually, running a system test involves the following steps:

1. Deploy the Elastic Stack, including Elasticsearch, Kibana, and the Elastic Agent. This step takes time so it should typically be done once as a pre-requisite to running system tests on multiple data streams.
1. Run a new Elastic Agent and enroll it with Fleet (running in the Kibana instance).
1. Depending on the Elastic Package whose data stream is being tested, deploy an instance of the package's integration service.
1. Create a test policy that configures a single data stream for a single package.
1. Assign the test policy to the enrolled Agent.
1. Wait a reasonable amount of time for the Agent to collect data from the
   integration service and index it into the correct Elasticsearch data stream.
1. Query the first 500 documents based on `@timestamp` for validation.
1. Validate mappings are defined for the fields contained in the indexed documents.
1. Validate that the JSON data types contained `_source` are compatible with
   mappings declared for the field.
1. Validate mappings generated after ingesting documents are valid according to the definitions installed by the package.
    - Requires `ELASTIC_PACKAGE_FIELD_VALIDATION_TEST_METHOD` to be unset or set to `mappings`.
1. If the Elastic Agent from the stack is not used, unenroll and remove the Elastic Agent as well as the test policies created.
1. Delete test artifacts and tear down the instance of the package's integration service.
1. Once the data stream have been system tested, unenroll and remove the Elastic Agent
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

If the package collects information from a running service, we can define the configuration for deploying it during system tests.
We can define it on either the package level:

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
* `agent` - (Deprecated) Custom `elastic-agent` with Docker Compose
* `k8s` - Kubernetes
* `tf` - Terraform

### Docker Compose service deployer

When using the Docker Compose service deployer, the `<service deployer files>` must include a `docker-compose.yml` file.
The `docker-compose.yml` file defines the integration service(s) for the package. If your package has a logs data stream,
the log files from your package's integration service must be written to a volume. For example, the `apache` package has
the following definition in it's integration service's `docker-compose.yml` file.

```yaml
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

```yaml
services:
  mysql:
    # Other properties such as build, ports, etc.
    volumes:
      # Other volumes.
      - mysqldata:/var/lib/mysql

volumes:
  mysqldata:
```

#### Run provisioner tool along with the Docker Compose service deployer

Along with the Docker Compose service deployer, other services could be added in the docker-compose scenario
to run other provisioner tools. For instance, the following example shows how Terraform could be used with the
Docker Compose service deployer, but other tools could be used too.

**Please note**: this is not officially supported by `elastic-package`. Package owners are responsible for maintaining
their own provisioner Dockerfiles and other resources required (e.g. scripts).

There is an example in the [test package `nginx_multiple_services`](../../test/packages/parallel/nginx_multiple_services/).

For that, you need to add another `terraform` service container in the docker-compose scenario as follows:
```yaml
version: '2.3'
services:
  nginx:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - 80
    volumes:
      - ${SERVICE_LOGS_DIR}:/var/log/nginx
    depends_on:
      terraform:
        condition: service_healthy
  terraform:
    tty: true
    stop_grace_period: 5m
    build:
      context: .
      dockerfile: Dockerfile.terraform
    environment:
      - TF_VAR_TEST_RUN_ID=${TEST_RUN_ID:-detached}
      - TF_VAR_CREATED_DATE=${CREATED_DATE:-unknown}
      - TF_VAR_BRANCH=${BRANCH_NAME_LOWER_CASE:-unknown}
      - TF_VAR_BUILD_ID=${BUILD_ID:-unknown}
      - TF_VAR_ENVIRONMENT=${ENVIRONMENT:-unknown}
      - TF_VAR_REPO=${REPO:-unknown}
    volumes:
      - ./tf/:/stage/
      - ${SERVICE_LOGS_DIR}:/tmp/service_logs/
```

Two other files required are to define this `terraform` service:
- Dockerfile to build the terraform container.
- Shellscript to trigger the terraform commands.

Those files are available in the [test package](../../test/packages/parallel/nginx_multiple_services/data_stream/access_docker_tf/_dev/deploy/docker/)
([Dockerfile.terraform](../../test/packages/parallel/nginx_multiple_services/data_stream/access_docker_tf/_dev/deploy/docker/Dockerfile.terraform) and
[terraform.run.sh](../../test/packages/parallel/nginx_multiple_services/data_stream/access_docker_tf/_dev/deploy/docker/terraform.run.sh))
and they could be used as a basis for other packages.

Along with that docker-compose definition, it is required to add the terraform files (`*.tf`) within the `docker` folder.
They can be placed in a `tf` folder for convenience.

This terraform container must be used as a helper for the main service defined in the docker-compose to create the required resources for the main service (e.g. `nginx` in the example above).
In this example, Terraform creates a new file in `/tmp/service_logs` and it is used by the `nginx` service.

Currently, there is no support to use outputs of terraform via this service deployer to set parameters in the test configuration file (variables),
as it is available in the terraform service deployer.


### Agent service deployer

**NOTE**: Deprecated in favor of creating [new Elastic Agents in each test](#running-a-system-test). These
Elastic Agents can be customized through the test configuration files adding the required settings. The settings
available are detailed in [this section](#test-case-definition).

When using the Agent service deployer, the `elastic-agent` provided by the stack
will not be used. An agent will be deployed as a Docker compose service named `docker-custom-agent`
which base configuration is provided [here](../../internal/install/_static/docker-custom-agent-base.yml).
This configuration will be merged with the one provided in the `custom-agent.yml` file.
This is useful if you need different capabilities than the provided by the
`elastic-agent` used by the `elastic-package stack` command.

`custom-agent.yml`
```yaml
services:
  docker-custom-agent:
    pid: host
    cap_add:
      - AUDIT_CONTROL
      - AUDIT_READ
    user: root
```

This will result in an agent configuration such as:

```yaml
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

```yaml
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
```Dockerfile
FROM docker.elastic.co/elastic-agent/elastic-agent-complete:8.4.0
USER root
RUN apt-get update && apt-get -y install \
    libaio1 \
    wget \
    unzip
```
An example for `custom-agent.yml` in multi-service setup is as below
```yaml
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

```hcl
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

#### Terraform Outputs

The outputs generated by the terraform service deployer can be accessed in the system test config using handlebars template.
For example, if a `SQS queue` is configured in terraform and if the `queue_url` is configured as output , it can be used in the test config as a handlebars template `{{TF_OUTPUT_queue_url}}`

Sample Terraform definition

```hcl
resource "aws_sqs_queue" "test" {

}

output "queue_url"{
  value = aws_sqs_queue.test.url
}
```

Sample system test config

```yaml
data_stream:
  vars:
    period: 5m
    latency: 10m
    queue_url: '{{TF_OUTPUT_queue_url}}'
    tags_filter: |-
      - key: Name
        value: "elastic-package-test-{{TEST_RUN_ID}}"
```

For complex outputs from terraform you can use `{{TF_OUTPUT_root_key.nested_key}}`

```hcl
output "root_key"{
  value = someoutput.nested_key_value
}
```
``` json
{
  "root_key": {
    "sensitive": false,
    "type": [
      "object",
      {
        "nested_key": "string"
      }
    ],
    "value": {
      "nested_key": "this is a nested key"
    }
  }
}
```
```yaml
data_stream:
  vars:
    queue_url: '{{TF_OUTPUT_root_key.nested_key}}'
```

#### Environment variables

To use environment variables within the Terraform service deployer a `env.yml` file is required.

The file should be structured like this:

```yaml
services:
  terraform:
    environment:
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
```

It's purpose is to inject environment variables in the Terraform service deployer environment.

To specify a default use this syntax: `AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID:-default}`, replacing `default` with the desired default value.

**NOTE**: Terraform requires to prefix variables using the environment variables form with `TF_VAR_`. These variables are not available in test case definitions because they are [not injected](https://github.com/elastic/elastic-package/blob/f5312b6022e3527684e591f99e73992a73baafcf/internal/testrunner/runners/system/servicedeployer/terraform_env.go#L43) in the test environment.

#### Cloud Provider CI support

Terraform is often used to interact with Cloud Providers. This requires Cloud Provider credentials.

In CI, these credentials are already available through the CI vault instance.

#### Tagging/labelling created Cloud Provider resources

Leveraging Terraform to create cloud resources is useful but risks creating leftover resources that are difficult to remove.
There is a CI pipeline in charge of looking for stale resources in the Cloud providers: https://buildkite.com/elastic/elastic-package-cloud-cleanup

There are some specific environment variables that should be leveraged to overcome this issue; these variables are already injected to be used by Terraform (through `TF_VAR_`):
- `TF_VAR_TEST_RUN_ID`: a unique identifier for the test run, allows to distinguish each run
- `BRANCH_NAME_LOWER_CASE`: the branch name or PR number the CI run is linked to
- `BUILD_ID`: incremental number providing the current CI run number
- `CREATED_DATE`: the creation date in epoch time, milliseconds, when the resource was created
- `ENVIRONMENT`: what environment created the resource (`ci`)
- `REPO`: the GitHub repository name (`elastic-package`)

### Kubernetes service deployer

The Kubernetes service deployer requires the `_dev/deploy/k8s` directory to be present. It can include additional `*.yaml` files to deploy
custom applications in the Kubernetes cluster (e.g. Nginx deployment). It is possible to use a `kustomization.yaml` file.
If no resource definitions (`*.yaml` files ) are needed,
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


### Defining more than one service deployer

Since `elastic-package` v0.113.0, it is allowed to define more than one service deployer in each `_dev/deploy` folder. And each system test
configuration can choose which service deployer to use among them.
For instance, a data stream could contain a definition for Docker Compose and a Terraform service deployers.

First, `elastic-package` looks for the corresponding `_dev/folder` to use. It will follow this order, and the first one that exists has
preference:
- Deploy folder at Data Stream level: `packages/<package_name>/data_stream/<data_stream_name>/_dev/deploy/`
- Deploy folder at Package level: `packages/<package_name>/data_stream/<data_stream_name>/_dev/deploy/`

If there is more than one service deployer defined in the deploy folder found, the system test configuration files of the
required tests must set the `deployer` field to choose which service deployer configure and start for that given test. If that setting
is not defined and there are more than one service edployer, `elastic-package` will fail with an error since it is not supported
to run several service deployers at the same time.

Example of system test configuration including `deployer` setting:

```yaml
deployer: docker
service: nginx
vars: ~
data_stream:
  vars:
    paths:
      - "{{SERVICE_LOGS_DIR}}/access.log*"
```

In this example, `elastic-package` looks for a Docker Compose service deployer in the given `_dev/deploy` folder found previously.

Each service deployer folder keep the same format and files as defined in previous sections.

For instance, this allows to test one data stream using different inputs, each input with a different service deployer. One of them could be using
the Docker Compose service deployer, and another input could be using terraform to create resources in AWS.

You can find an example of a package using this in this [test package](../../test/packages/parallel/nginx_multiple_services/).


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
| agent.linux_capabilities | array string | | Linux Capabilities that must be enabled in the system to run the Elastic Agent process. |
| agent.pid_mode | string | | Controls access to PID namespaces. When set to `host`, the agent will have access to the PID namespace of the host. |
| agent.ports | array string | | List of ports to be exposed to access to the Elastic Agent.|
| agent.runtime | string | | Runtime to run Elastic Agent process. |
| agent.pre_start_script.language | string | | Programming language of the pre-start script, executed before starting the agent. Currently, only `sh` is supported.|
| agent.pre_start_script.contents | string | | Code to run before starting the agent. |
| agent.provisioning_script.language | string | | Programming language of the provisioning script. Default: `sh`. |
| agent.provisioning_script.contents | string | | Code to run as a provisioning script to customize the system where the agent will be run. |
| agent.user | string | | User that runs the Elastic Agent process. |
| assert.hit_count | integer |  | Exact number of documents to wait for being ingested. |
| assert.min_count | integer |  | Minimum number of documents to wait for being ingested. |
| assert.fields_present | []string|  | List of fields that must be present in the documents to stop waiting for new documents. |
| data_stream.vars | dictionary |  | Data stream level variables to set (i.e. declared in `package_root/data_stream/$data_stream/manifest.yml`). If not specified the defaults from the manifest are used. |
| deployer | string|  | Name of the service deployer to setup for this system test. Available values: docker, tf or k8s. |
| ignore_service_error | boolean | no | If `true`, it will ignore any failures in the deployed test services. Defaults to `false`. |
| input | string | yes | Input type to test (e.g. logfile, httpjson, etc). Defaults to the input used by the first stream in the data stream manifest. |
| numeric_keyword_fields | []string |  | List of fields to ignore during validation that are mapped as `keyword` in Elasticsearch, but their JSON data type is a number. |
| policy_template | string |  | Name of policy template associated with the data stream and input. Required when multiple policy templates include the input being tested. |
| service | string |  | Name of a specific Docker service to setup for the test. |
| service_notify_signal | string |  | Signal name to send to 'service' when the test policy has been applied to the Agent. This can be used to trigger the service after the Agent is ready to receive data. |
| skip.link | URL |  | URL linking to an issue about why the test is skipped. |
| skip.reason | string |  | Reason to skip the test. If specified the test will not execute. |
| skip_ignored_fields | array string |  | List of fields to be skipped when performing validation of fields ignored during ingestion. |
| skip_transform_validation | boolean |  | Disable or enable the transforms validation performed in system tests. |
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

#### Available assertions to wait for documents

System tests allow to define different conditions to collect data from the integration service and index it into the correct Elasticsearch data stream.

By default, `elastic-package` waits until there are more than zero documents ingested. The exact number of documents to be
validated in this default scenario depends on how fast the documents are ingested.

There are other 4 options available:
- Wait for collecting exactly `assert.hit_count` documents into the data stream.
    - It will fail if the final number of documents ingested into Elasticsearch is different from `assert.hit_count` documents.
- Wait for collecting at least `assert.min_count` documents into the data stream.
    - Once there have been `assert.min_count` or more documents ingested, `elastic-package` will proceed to validate the documents.
    - This could be used to ensure that a wide range of different documents have been ingested into Elasticsearch.
- Collect data into the data stream until all the fields defined in the list `assert.fields_present` are present in any of the documents.
    - Each field in that list could be present in different documents.

The following example shows how to add an assertion on the number of hits in a given system test using `assert.hit_count`.

Consider this example from the `httpjson/generic` data stream's `test-expected-hit-count-config.yml`, shown below.

```yaml
input: httpjson
service: httpjson
data_stream:
  vars:
    data_stream.dataset: httpjson.generic
    username: test
    password: test
    request_url: http://{{Hostname}}:{{Port}}/testexpectedhits/api
    response_split: |-
      target: body.hits
      type: array
      keep_parent: false
assert:
  hit_count: 3
```

The `data_stream.vars.request_url` corresponds to a test-stub path in the `_dev/deploy/docker/files/config.yml` file.

```yaml
  - path: /testexpectedhits/api
    methods: ["GET"]
    request_headers:
      Authorization:
        - "Basic dGVzdDp0ZXN0"
    responses:
      - status_code: 200
        headers:
          Content-Type:
            - "application/json; charset=utf-8"
        body: |-
          {"total":3,"hits":[{"message": "success"},{"message": "success"},{"message": "success"}]}
```

Handlebar syntax in `httpjson.yml.hbs`

```handlebars
{{#if response_split}}
response.split:
  {{response_split}}
{{/if}}
```

inserts the value of `response_split` from the test configuration into the integration, in this case, ensuring the `total.hits[]` array from the test-stub response yields 3 hits.

Returning to `test-expected-hit-count-config.yml`, when `assert.hit_count` is defined and `> 0` the test will assert that the number of hits in the array matches that value and fail when this is not true.

#### Defining new Elastic Agents for a given test

System tests allow to create specific an Elsatic Agent for each test with custom settings or additional software.
Elastic Agents can be customized by defining the needed `agent.*` settings.

As an example to add settings to create a new Elastic Agent in a given test,
the`auditd_manager/audtid` data stream's `test-default-config.yml` is shown below:

```yaml
data_stream:
  vars:
    audit_rules:
      - "-a always,exit -F arch=b64 -S execve,execveat -k exec"
    preserve_original_event: true
agent:
  runtime: docker
  pid_mode: "host"
  linux_capabilities:
    - AUDIT_CONTROL
    - AUDIT_READ
```

With this test configuration file, the Elastic Agent is running in a docker environment,
turned on sharing between container and the host operating system the PID address space as well as
the Linux Capabilities `AUDIT_CONTROL` and `AUDIT_READ` have been enabled for that Elastic Agent.
In [this section](#running-a-system-test), there is also another example to customize the scripts
to install new software or define new environment variables in the Elastic Agents.

#### Placeholders

The `SERVICE_LOGS_DIR` placeholder is not the only one available for use in a data stream's `test-<test_name>-config.yml` file. The complete list of available placeholders is shown below.

| Placeholder name | Data type | Description |
| --- | --- | --- |
| `Hostname`| string | Addressable host name of the integration service. |
| `Ports` | []int | Array of addressable ports the integration service is listening on. |
| `Port` | int | Alias for `Ports[0]`. Provided as a convenience. |
| `Logs.Folder.Agent` | string | Path to integration service's logs folder, as addressable by the Agent. |
| `SERVICE_LOGS_DIR` | string | Alias for `Logs.Folder.Agent`. Provided as a convenience. |
| `TEST_RUN_ID` | string | Unique identifier for the test run, allows to distinguish each run. Provided as a convenience. |

Placeholders used in the `test-<test_name>-config.yml` must be enclosed in `{{{` and `}}}` delimiters, per Handlebars syntax.


**NOTE**: Terraform variables in the form of environment variables (prefixed with `TF_VAR_`) are not injected and cannot be used as placeholder (their value will always be empty).

## Global test configuration

Each package could define a configuration file in `_dev/test/config.yml` that allows to:
- skip all the system tests defined.
- set if these system tests should be running in parallel or not.

```yaml
system:
  parallel: true
  skip:
    reason: <reason>
    link: <link_to_issue>
```

## Running a system test

Once the two levels of configurations are defined as described in the previous section, you are ready to run system tests for a package's data streams.

First you must deploy the Elastic Stack. This corresponds to steps 1 and 2 as described in the [_Conceptual process_](#Conceptual-process) section.

```shell
elastic-package stack up -d
```

For a complete listing of options available for this command, run `elastic-package stack up -h` or `elastic-package help stack up`.

Next, you must invoke the system tests runner. This corresponds to steps 3 through 7 as described in the [_Conceptual process_](#Conceptual-process) section.

If you want to run system tests for **all data streams** in a package, navigate to the package's root folder (or any sub-folder under it) and run the following command.

```shell
elastic-package test system
```

If you want to run system tests for **specific data streams** in a package, navigate to the package's root folder (or any sub-folder under it) and run the following command.

```shell
elastic-package test system --data-streams <data stream 1>[,<data stream 2>,...]
```

Finally, when you are done running all system tests, bring down the Elastic Stack. This corresponds to step 8 as described in the [_Conceptual process_](#Conceptual_process) section.

```shell
elastic-package stack down
```

Starting with [elastic-package version `v0.101.0`](https://github.com/elastic/elastic-package/releases/tag/v0.101.0),
by default, running `elastic-package test system` command will setup a new Elastic Agent for each test defined in the package.

For each system test configuration file defined in each data stream, a new Elastic Agent is going
to be started and enrolled in a given test Agent Policy in Fleet specifically to run those system tests.
Once those tests have finished, this Elastic Agent is going to be unenrolled and removed from
Fleet (along with the Agent Policies) and a new Elastic Agent will be created for the next test configuration
file.

These Elastic Agents can be customized adding the required settings for the tests in the test configuration file.
For example, the `oracle/memory` data stream's [`test-memory-config.yml`](https://github.com/elastic/elastic-package/blob/19b2d35c0d7aea7357ccfc572398f39812ff08bc/test/packages/parallel/oracle/data_stream/memory/_dev/test/system/test-memory-config.yml) is shown below:
```yaml
vars:
  hosts:
    - "oracle://sys:Oradoc_db1@{{ Hostname }}:{{ Port }}/ORCLCDB.localdomain?sysdba=1"
agent:
  runtime: docker
  provisioning_script:
    language: "sh"
    contents: |
      set -eu
      if grep wolfi /etc/os-release > /dev/null ; then
        apk update && apk add libaio wget unzip
      else
        apt-get update && apt-get -y install libaio1 wget unzip
      fi
      mkdir -p /opt/oracle
      cd /opt/oracle
      wget https://download.oracle.com/otn_software/linux/instantclient/214000/instantclient-basic-linux.x64-21.4.0.0.0dbru.zip && unzip -o instantclient-basic-linux.x64-21.4.0.0.0dbru.zip || exit 1
      wget https://download.oracle.com/otn_software/linux/instantclient/217000/instantclient-sqlplus-linux.x64-21.7.0.0.0dbru.zip && unzip -o instantclient-sqlplus-linux.x64-21.7.0.0.0dbru.zip || exit 1
      mkdir -p /etc/ld.so.conf.d/
      echo /opt/oracle/instantclient_21_4 > /etc/ld.so.conf.d/oracle-instantclient.conf && ldconfig || exit 1
      cp /opt/oracle/instantclient_21_7/glogin.sql /opt/oracle/instantclient_21_7/libsqlplus.so /opt/oracle/instantclient_21_7/libsqlplusic.so /opt/oracle/instantclient_21_7/sqlplus /opt/oracle/instantclient_21_4/
  pre_start_script:
    language: "sh"
    contents: |
      export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:-""}:/opt/oracle/instantclient_21_4"
      export PATH="${PATH}:/opt/oracle/instantclient_21_7:/opt/oracle/instantclient_21_4"
      cd /opt/oracle/instantclient_21_4
```

**IMPORTANT**: The provisioning script must exit with a code different from zero in case any of the commands defined fails.
That will ensure that the docker build step run by `elastic-package` fails too.

Another example setting capabilities to the agent ([`auditid_manager` test package](https://github.com/elastic/elastic-package/blob/6338a33c255f8753107f61673245ef352fbac0b6/test/packages/parallel/auditd_manager/data_stream/auditd/_dev/test/system/test-default-config.yml)):
```yaml
data_stream:
  vars:
    audit_rules: |-
      ...
    preserve_original_event: true
agent:
  runtime: docker
  pid_mode: "host"
  linux_capabilities:
    - AUDIT_CONTROL
    - AUDIT_READ
```

Considerations for packages using the [Agent service deployer](#agent-service-deployer) (`_dev/deploy/agent` folder):
- If `_dev/deploy/agent` folder, `elastic-package` will continue using the Elastic Agent as described in [section](#agent-service-deployer).
- If a package that defines the agent service deployer (`agent` folder) wants to stop using this Agent Service deployer, these would be the steps:
    - Create a new `_dev/deploy/docker` adding the service container if needed.
    - Define the settings required for your Elastic Agents in all the test configuration files.

### Running system tests with the Elastic Agents from the stack

Before [elastic-package version `v0.101.0`](https://github.com/elastic/elastic-package/releases/tag/v0.101.0),
by default, running `elastic-package test system` command will use the Elastic Agent:
- created and enrolled when the Elastic stack is started (running `elastic-package stack up`), or
- created with other service deployers (kubernetes or custom agents deployers).

If needed, this can be enabled by setting the environment variable `ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT=false`, so
`elastic-package` would use the Elastic Agent spin up along with the Elastic stack. Example:

```
  elastic-package stack up -v -d
  cd /path/package
  ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT=false elastic-package test system -v
  elastic-package stack down -v
```

When running system tests with the Elastic Agent from the stack, all tests defined in the package are going to be run using
the same Elastic Agent from the stack. That Elastic Agent is not going to be stopped/unenroll between different execution tests.


### Generating sample events

As the system tests exercise an integration end-to-end from running the integration's service all the way
to indexing generated data from the integration's data streams into Elasticsearch, it is possible to generate
`sample_event.json` files for each of the integration's data streams while running these tests.

```shell
elastic-package test system --generate
```

### System testing negative or false-positive scenarios

The system tests support packages to be tested for negative scenarios. An example would be to test that the `assert.hit_count` is verified when all the docs are ingested rather than just finding enough docs for the testcase.

There are some special rules for testing negative scenarios

- The negative / false-positive test packages are added under `test/packages/false_positives`
- It is required to have a file `<package_name>.expected_errors` with the lines needed for every package added under `test/packages/false_positives`.
- One line per error, taking into account that all `\n` were removed, meaning it is just one line for everything.
- As it is used `grep` with `-E` some kind of regexes can be used.

Example `expected_errors` file content:

```xml
<testcase name=\"system test: pagination\" classname=\"httpjson_false_positive_asserts.generic\" time=\".*\"> * <failure>observed hit count 4 did not match expected hit count 2</failure>
```

### System testing with logstash

It is possible to test packages that output to Logstash which in turn publishes events to Elasticsearch.
A profile config option `stack.logstash_enabled` has been added to profile configuration.

When this profile config is enabled
- Logstash output is added in Fleet with id `fleet-logstash-output`
- Logstash service is created in the stack which reads from `elastic-agent` input and outputs to `elasticsearch`.
- Logstash is also configured with `elastic-integration` plugin. Once configured to point to an Elasticsearch cluster, this filter will detect which ingest pipeline (if any) should be executed for each event, auto-detecting the event’s data-stream and its default pipeline.

A sample workflow would look like:

- You can [create](https://github.com/elastic/elastic-package#elastic-package-profiles-create) a new profile / [use existing profile](https://github.com/elastic/elastic-package#elastic-package-profiles-use) to test this.
- Navigate to `~/.elastic-package/profiles/<profilename>/`.
- Rename `config.yml.example` to `config.yml` [ If config is not used before ]
- Add the following line (or uncomment if present) `stack.logstash_enabled: true`
- Run `elastic-package stack up -d -v`
- Navigate to the package folder in integrations and run `elastic-package test system -v`

### Running system tests without cleanup (technical preview)

By default, `elastic-package test system` command always performs these steps to run tests for a given package:
1. Setup:
    - Start the service to be tested with a given configuration (`data_stream/<name>/_dev/test/sytem/test-name-config.yml`) and variant (`_dev/deploy/variants.yml`).
    - Build and install the package
    - Create the required resources in Elasticsearch to configure the ingestion of data through the Elastic Agent.
        - test policy
        - add package data stream to the test policy
    - Ensure agent is enrolled.
    - Ensure that the data stream has no old documents.
    - Assign policy to the agent.
    - Ensure there are hits received in Elasticsearch in the package data stream.
2. Run the tests (validate fields, check transforms, etc.)
    - Validate fields (e.g. mappings)
    - Assert number of hit counts.
    - Check transforms.
3. Tear Down:
    - Rollback all the changes in Elasticsearch:
        - Restore previous policy to the agent.
        - Delete the test policy.
        - Uninstall the package.
    - Stop the service (e.g. container) to be tested.
    - Wipe all the documents of the data stream.

This process is repeated for each combination of:
- data stream `D`, if the package is of `integration` type.
- configuration file defined under `_dev/test/sytem` folder.
- variant (`_dev/deploy/variants.yml`).

It's possible also to run these steps independently. For that it is required to set which configuration file (`--config-file`)
and which variant (`--variant`), if any, is going to be used to start and configure the service for these tests.

**NOTE**: Currently, there is just support for packages using the following service deployers: `docker`, `agent` (custom agents) and `k8s`.

Then, each step can be run using one of these flags:
- Run the setup (`--setup`), after this command is executed:
    - Service container will be kept running.
    - Elastic Agent is going to be sending documents to Elasticsearch.
- Run just the tests (`--no-provision`), after this command is executed:
    - Service container will not be stopped.
    - Documents in the Data stream will be deleted, so the tests will use the latest documents sent.
    - Elastic Agent is going to be sending documents to Elasticsearch.
    - NOTE: This command can be run several times.
- Run just the setup (`--tear-down`), after this command:
    - Service container is going to be stopped.
    - All changes in Elasticsearch are rollback.
    - Package Data stream is deleted.


Examples:

```shell
# Start Elastic stack as usual
elastic-package stack up -v -d

# Testing a package using a configuration file and a variant (e.g. mysql)
# variant name is just required in --setup flag
elastic-package test system -v --config-file data_stream/status/_dev/test/system/test-default-config.yml --variant percona_8_0_36 --setup
elastic-package test system -v --no-provision
elastic-package test system -v --tear-down

# Testing a package using a configuration file (no variants defined in the package)
elastic-package test system -v --config-file data_stream/audit/_dev/test/system/test-tcp-config.yml --setup
elastic-package test system -v --no-provision
elastic-package test system -v --tear-down

# Testing a input package using a configuration file (no variants defined in the package)
elastic-package test system -v --config-file _dev/test/system/test-mysql-config.yml --setup
elastic-package test system -v --no-provision
elastic-package test system -v --tear-down

elastic-package stack down -v
```

#### Running system tests in parallel (technical preview)

By default, `elatic-package` runs every system test defined in the package sequentially.
This could be changed to allow running in parallel tests. For that it is needed that:
- system tests cannot be run using the Elastic Agent from the stack.
- package must define the global test configuration file with these contents to enable system test parallelization:
  ```yaml
  system:
    parallel: true
  ```
- define how many tests in parallel should be running
    - This is done defining the environment variable `ELASTIC_PACKAGE_MAXIMUM_NUMBER_PARALLEL_TESTS`


Given those requirements, this is an example to run system tests in parallel:
```shell
ELASTIC_PACKAGE_MAXIMUM_NUMBER_PARALLEL_TESTS=5 \
  ELASTIC_PACKAGE_TEST_ENABLE_INDEPENDENT_AGENT=true \
  elastic-package test system -v
```

**NOTE**:
- Currently, just system tests support to run tests in parallel.
- **Not recommended** to enable system tests in parallel for packages that make use of the Terraform or Kubernetes service deployers.

### Detecting ignored fields

As part of the system test, `elastic-package` checks whether any documents couldn't successfully map any fields. Common issues are the configured field limit being exceeded or keyword fields receiving values longer than `ignore_above`. You can learn more in the [Elasticsearch documentation](https://www.elastic.co/guide/en/elasticsearch/reference/current/mapping-ignored-field.html).

In this case, `elastic-package test system` will fail with an error and print a sample of affected documents. To fix the issue, check which fields got ignored and the `ignored_field_values` and either adapt the mapping or the ingest pipeline to accommodate for the problematic values. In case an ignored field can't be meaningfully mitigated, it's possible to skip the check by listing the field under the `skip_ignored_fields` property in the system test config of the data stream:
```
# data_stream/<data stream name>/_dev/test/system/test-default-config.yml
skip_ignored_fields:
  - field.to.ignore
```

## Continuous Integration

`elastic-package` runs a set of system tests on some [dummy packages](https://github.com/elastic/elastic-package/tree/main/test/packages) to ensure it's functionalities work as expected. This allows to test changes affecting package testing within `elastic-package` before merging and releasing the changes.

Tests use set of environment variables that are set at the beginning of the `Jenkinsfile`.

The exposed environment variables are passed to the test runners through service deployer specific configuration (refer to the service deployer section for further details).

### Stack version

The tests use the [default version](https://github.com/elastic/elastic-package/blob/main/internal/install/stack_version.go#L9) `elastic-package` provides.

You can override this value by changing it in your PR if needed. To update the default version always create a dedicated PR.
