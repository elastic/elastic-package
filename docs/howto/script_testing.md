# HOWTO: Writing script tests for a package

Script testing is an advanced topic that assumes knowledge of [pipeline](./pipeline_testing.md)
and [system](./system_testing.md) testing.

Testing packages with script testing is only intended for testing cases that
cannot be adequately covered by the pipeline and system testing tools such as
testing failure paths and package upgrades. It can also be used for debugging
integrations stack issues.

## Introduction

The script testing system is build on the Go testscript package with extensions
provided to allow scripting of stack and integration operations such as
bringing up a stack, installing packages and running agents. For example, using
these commands it is possible to express a system test as described in the
system testing [Conceptual Process](./system_testing.md#conceptual-process) section.


## Expressing tests

Tests are written as [txtar format](https://pkg.go.dev/golang.org/x/tools/txtar#hdr-Txtar_format)
files in a data stream's \_dev/test/scripts directory. The logic for the test is
written in the txtar file's initial comment section and any additional resource
files are included in the txtar file's files sections.

The standard commands and behaviors for testscript scripts are documented in
the [testscript package documentation](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript).


## Extension commands

The test script command provides additional commands to aid in interacting with
a stack, starting agents and services and validating results.

- `sleep`: sleep for a duration (Go `time.Duration` parse syntax)
- `date`: print the current time in RFC3339, optionally setting a variable with the value
- `GET`: perform an HTTP GET request, emitting the response body to stdout
- `POST`: perform an HTTP POST request, emitting the response body to stdout
- `match_file`: perform a grep pattern match between a pattern file and a data file

- stack commands:
  - `stack_up`: bring up a version of the Elastic stack
  - `use_stack`: use a running Elastic stack
  - `stack_down`: take down a started Elastic stack
  - `dump_logs`: dump the logs from the stack into a directory
  - `get_policy`: print the details for a policy

- agent commands:
  - `install_agent`: install an Elastic Agent policy
  - `uninstall_agent`: remove an installed Elastic Agent policy

- package commands:
  - `add_package`: add the current package's assets
  - `remove_package`: remove assets for the current package
  - `add_package_zip`: add assets from a Zip-packaged integration package
  - `remove_package_zip`: remove assets for Zip-packaged integration package
  - `upgrade_package_latest`: upgrade the current package or another named package to the latest version

- data stream commands:
  - `add_data_stream`: add a data stream policy
  - `remove_data_stream`: remove a data stream policy
  - `get_docs`: get documents from a data stream

- docker commands:
  - `docker_up`: start a docker service
  - `docker_down`: stop a started docker service and print the docker logs to stdout
  - `docker_signal`: send a signal to a running docker service
  - `docker_wait_exit`: wait for a docker service to exit 

- pipeline commands:
  - `install_pipelines`: install ingest pipelines from a path
  - `simulate`: run a pipeline test
  - `uninstall_pipelines`: remove installed ingest pipelines


## Environment variables

- `CONFIG_ROOT`: the `elastic-package` configuration root path
- `CONFIG_PROFILES`: the `elastic-package` profiles configuration root path
- `HOME`: the user's home directory path
- `PKG`: the name of the running package
- `PKG_ROOT`: the path to the root of the running package
- `CURRENT_VERSION`: the current version of the package
- `PREVIOUS_VERSION`: the previous version of the package
- `DATA_STREAM`: the name of the data stream
- `DATA_STREAM_ROOT`: the path to the root of the data stream


## Conditions

The testscript package allows conditions to be set that allow conditional
execution of commands. The test script command adds a condition that reflects
the state of the `--external-stack` flag. This allows tests to be written that
conditionally use either an externally managed stack, or a stack that has been
started by the test script.


## Example

As an example, a basic system test could be expressed as follows.
```
# Only run the test if --external-stack=true.
[!external_stack] skip 'Skipping external stack test.'
# Only run the test if the jq executable is in $PATH. This is needed for a test below.
[!exec:jq] skip 'Skipping test requiring absent jq command'

# Register running stack.
use_stack -profile ${CONFIG_PROFILES}/default

# Install an agent.
install_agent -profile ${CONFIG_PROFILES}/default NETWORK_NAME

# Bring up a docker container.
#
# The service is described in the test-hits/docker-compose.yml below with
# its logs in test-hits/logs/generated.log.
docker_up -profile ${CONFIG_PROFILES}/default -network ${NETWORK_NAME} test-hits

# Add the package resources.
add_package -profile ${CONFIG_PROFILES}/default

# Add the data stream.
#
# The configuration for the test is described in test_config.yaml below.
add_data_stream -profile ${CONFIG_PROFILES}/default test_config.yaml DATA_STREAM_NAME

# Start the service.
docker_signal test-hits SIGHUP

# Wait for the service to exit.
docker_wait_exit -timeout 5m test-hits

# Check that we can see our policy.
get_policy -profile ${CONFIG_PROFILES}/default -timeout 1m ${DATA_STREAM_NAME}
cp stdout got_policy.json
exec jq '.name=="'${DATA_STREAM_NAME}'"' got_policy.json
stdout true

# Take down the service and check logs for our message.
docker_down test-hits
! stderr .
stdout '"total_lines":10'

# Get documents from the data stream.
get_docs -profile ${CONFIG_PROFILES}/default -want 10 -timeout 5m ${DATA_STREAM_NAME}
cp stdout got_docs.json

# Remove the data stream.
remove_data_stream -profile ${CONFIG_PROFILES}/default ${DATA_STREAM_NAME}

# Uninstall the agent.
uninstall_agent -profile ${CONFIG_PROFILES}/default -timeout 1m 

# Remove the package resources.
remove_package -profile ${CONFIG_PROFILES}/default

-- test-hits/docker-compose.yml --
version: '2.3'
services:
  test-hits:
    image: docker.elastic.co/observability/stream:v0.20.0
    volumes:
      - ./logs:/logs:ro
    command: log --start-signal=SIGHUP --delay=5s --addr elastic-agent:9999 -p=tcp /logs/generated.log
-- test-hits/logs/generated.log --
ntpd[1001]: kernel time sync enabled utl
restorecond: : Reset file context quasiarc: liqua
auditd[5699]: Audit daemon rotating log files
anacron[5066]: Normal exit ehend
restorecond: : Reset file context vol: luptat
heartbeat: : <<eumiu.medium> Processing command: accept
restorecond: : Reset file context nci: ofdeFin
auditd[6668]: Audit daemon rotating log files
anacron[1613]: Normal exit mvolu
ntpd[2959]: ntpd gelit-r tatno
-- test_config.yaml --
input: tcp
vars: ~
data_stream:
  vars:
    tcp_host: 0.0.0.0
    tcp_port: 9999
```

Other complete examples can be found in the [with_script test package](https://github.com/elastic/elastic-package/blob/main/test/packages/other/with_script/data_stream/first/_dev/test/scripts).


## Running script tests

The `elastic-package test script` command has the following sub-command-specific
flags:

- `--continue`: continue running the script if an error occurs
- `--data-streams`: comma-separated data streams to test
- `--external-stack`: use external stack for script tests (default true)
- `--run`: run only tests matching the regular expression
- `--scripts`: path to directory containing test scripts (advanced use only)
- `--update`: update archive file if a cmp fails
- `--verbose-scripts`: verbose script test output (show all script logging)
- `--work`: print temporary work directory and do not remove when done


## Limitations

While the testscript package allows reference to paths outside the configuration
root and the package's root, the backing `elastic-package` infrastructure does
not, so it is advised that tests only refer to paths within the `$WORK` and
`$PKG_ROOT` directories.