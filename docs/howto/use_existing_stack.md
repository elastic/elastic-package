# HOWTO: Use `elastic-package` with an existing stack


## Introduction

`elastic-package` supports the use of environment variables to customize how it
interacts with the environment where it is running. Some of these variables can
be used to define the target Elastic Stack for many of its subcommands.

There are some commands that only need to reach Elasticsearch or Kibana, for
these commands you only need to configure the environment variables required to
configure these services. In other cases elastic-package needs to be able to
enroll agents, specially for system testing, for these cases you can use
the `environment` stack provider.


### Environment variables for existing stacks

Some `elastic-package` subcommands only need access to Elasticsearch or Kibana,
for them it is enough with setting some environment variables to define how to
reach them.
This is the case of subcommands such as `install`, `dump` or `export`, that use
the APIs to do their work, but don't need to manage Elastic Agents or a Fleet
Server.

Two environment variables are required to tell elastic-package about the target
Elasticsearch and Kibana hosts:
- `ELASTIC_PACKAGE_ELASTICSEARCH_HOST` for Elasticsearch.
- `ELASTIC_PACKAGE_KIBANA_HOST` for Kibana.

`elastic-package` can connect to services exposed through HTTPS or HTTP. If you
are using HTTPS with self-signed certificates you can set
`ELASTIC_PACKAGE_CA_CERT` to the path of the certificate of your CA. 

If your stack requires authentication, you can use the following environment
variables:
- `ELASTIC_PACKAGE_ELASTICSEARCH_API_KEY` for authentication based on API keys.
- `ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME` and
  `ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD` for basic authentication.

You can read more about the available environment variables in the [README](https://github.com/elastic/elastic-package/blob/main/README.md#useful-environment-variables).


### The `environment` stack provider

Some `elastic-package` subcommands expect some Fleet configuration, a running
Fleet Server, and to be able to manage Elastic Agents. This is specially the
case of system tests and benchmarks.
For these cases you can use the `environment` stack provider, that takes care of
filling these gaps for a running stack.
You can also use this provider to setup Fleet, with a running agent, for any
existing stack.

The `environment` provider runs Elastic Agents and other services as local
containers using Docker Compose. These services run on their own networks, so
they cannot access Elasticsearch and Kibana listening only on `localhost`.
If you want to use `elastic-package` with a stack running natively on localhost,
you will need to configure it to listen in all interfaces (`0.0.0.0`).

:warning: The `environment` provider modifies the Fleet configuration of the
target stack. Avoid using it in environments that you use for other purpouses,
specially in production environments. :warning:

To use the `environment` provider with an existing stack, setup the environment
variables as described in the previous section, and then run:
```sh
elastic-package stack up -v -d --provider environment
```

After this command finishes succesfully, your environment will be ready to use
with Fleet and it will have an enrolled Elastic Agent. `elastic-package` will be
configured to run any of its commands with it.

To clean up everything, run:
```sh
elastic-package stack down
```

You can have multiple stacks configured, with different providers, if you use
profiles. You can read more about them in the [README](https://github.com/elastic/elastic-package/blob/main/README.md#elastic-package-profiles-1).


### Example: Using elastic-package with Kibana development environment

One of the use cases of the environment provider is to be able to use
`elastic-package` with other development environments, as could be the Kibana
development environment.

Once you have a working [Kibana development environment](https://github.com/elastic/kibana/blob/main/CONTRIBUTING.md),
you can follow the instructions described in this document.

First you need to ensure that Elasticsearch and Kibana are listening on all
interfaces, so containers can connect:
```sh
yarn es snapshot --license trial -E network.host=0.0.0.0 -E discovery.type=single-node
yarn start --host 0.0.0.0
```

Take note of the address logged when starting kibana, on this document we are
assuming that it is `http://localhost:5601/xyz`.

Then configure the required environment variables:
```sh
export ELASTIC_PACKAGE_KIBANA_HOST=http://localhost:5601/xyz
export ELASTIC_PACKAGE_ELASTICSEARCH_HOST=http://localhost:9200
export ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME=elastic
export ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD=changeme
```

And finally run elastic-package with the `environment` provider:
```sh
elastic-package stack up -v -d --provider environment
```
