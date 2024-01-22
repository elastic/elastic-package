# Use elastic-package to generate sample data

`elastic-package` with integrations can be used to generate sample data for integration packages. The following is a quick guide on how to use it with certain packages with your own stack deployment.

## Getting started

### Install elastic-package

The first thing needed, is an installation of `elastic-package`, you can find [here](https://github.com/elastic/elastic-package/tree/main?tab=readme-ov-file#getting-started) different ways how to install it.

### Setup Elastic Stack

As soon as elastic-package is set up, you need a running Elastic. The simplest thing to set it up with Docker is running `elastic-package stack up --version=8.13.0-SNAPSHOT -v`. This will start a snapshot build of an Elastic Stack cluster on your machine and you can open Kibana under `https://localhost:5601`.

If you have your own Cloud setup or run Kibana from source, you can set the environment variables accordingly. The following is a sample assuming you are running Kibana and Elasticsearch from source: 

```
ELASTIC_PACKAGE_ELASTICSEARCH_HOST=http://localhost:9200
ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME=elastic
ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD=changeme
ELASTIC_PACKAGE_KIBANA_HOST=http://localhost:5601
#ELASTIC_PACKAGE_CA_CERT
```

The same can be use to connect to your Elastic Cloud cluster. Unfortunately elastic-package currently does not support an API key to be used.

If you are running Kibana from source and have a base path like `ciy`, use the following HOST for Kibana:

```
ELASTIC_PACKAGE_KIBANA_HOST=http://localhost:5601/ciy
```

### Checkout integrations repository

To ingest data for an integration, currently you have to checkout the integrations repo to your machine as the templates for the data are not part of the packages itself. Clone the integrations repo to your disk:

```
git clone https://github.com/elastic/integrations.git
```

### Ingest data

Now jump to one of the packages which have a template file. We will be using nginx for this example but k8s and aws package also have templates inside. You need to `cd` into the nginx package:

```
cd packages/nginx
```

Inside the directory, run the following command with the `elastic-package` stack setup.

```
elastic-package benchmark stream -v
```

When running Kibana from source with `--no-base-path`, you can run the following command:

```
ELASTIC_PACKAGE_ELASTICSEARCH_HOST=http://localhost:9200 ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME=elastic ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD=changeme ELASTIC_PACKAGE_KIBANA_HOST=http://localhost:5601 elastic-package benchmark stream -v
```


The above command will backfill the last 15 minutes in your Elastic Stack with nginx metrics and logs and install the nginx integration. You can go to [https://localhost:5601/app/observability-log-explorer/](https://localhost:5601/app/observability-log-explorer/) and filter down on `Nginx -> Error Logs` to see the error logs that are ingested.



## Links

There is a lot more documentation around how to build your own templates for packages or adjust existing ones. These are linked below:

* [Writing rally benchmarks for a package](https://github.com/elastic/elastic-package/blob/main/docs/howto/rally_benchmarking.md)