# Use your own custom images

By default, `elastic-package stack up` command starts the required docker images for each service.

Setting `--version` flag, you can start the Elastic stack using different versions, for instance SNAPSHOT version (e.g. 8.8.2-SNAPSHOT).
If this flag is not set, the default version used is by elastic-package is defined [here](../../internal/install/stack_version.go).

There could be cases where you need to use your own custom images for development or debugging purposes.
If you need to use your own custom docker images for your service (e.g. elastic-agent), this could be
achieved in two different ways:
- [defining environment variables](#using-environment-variables)
- [updating elastic-package configuration file](#using-the-configuration-file)

If both ways are used, environment variables have preference to set the custom image.

The current images that could be overwritten are:

| Service | Environment Variable | Entry configuration file |
| --- | --- | --- |
| Elasticsearch | ELASTICSEARCH_IMAGE_REF_OVERRIDE | elasticsearch |
| Kibana | KIBANA_IMAGE_REF_OVERRIDE | kibana |
| Elastic Agent | ELASTIC_AGENT_IMAGE_REF_OVERRIDE | elastic-agent |
| Logstash | LOGSTASH_IMAGE_REF_OVERRIDE | logstash |
| IsReady | ISREADY_IMAGE_REF_OVERRIDE | is_ready |


For the following two examples, it will be used as example overwriting elastic-agent image.
You can find here the instructions to create the docker images for the examples ([docs link](https://github.com/elastic/elastic-agent#packaging)):

```bash
# Go to elastic-agent repository and create the packages for docker
git checkout 8.8 # or the required tag like v8.8.*
DEV=true SNAPSHOT=true EXTERNAL=true PLATFORMS=linux/amd64 PACKAGES=docker  mage -v package

docker tag docker.elastic.co/beats/elastic-agent-complete:8.8.2-SNAPSHOT docker.elastic.co/beats/elastic-agent-complete:8.8.2-SNAPSHOT-test

git checkout v8.8.1
DEV=true SNAPSHOT=true EXTERNAL=true PLATFORMS=linux/amd64 PACKAGES=docker  mage -v package

docker tag docker.elastic.co/beats/elastic-agent-complete:8.8.1-SNAPSHOT docker.elastic.co/beats/elastic-agent-complete:8.8.1-SNAPSHOT-test
docker tag docker.elastic.co/beats/elastic-agent-complete:8.8.1-SNAPSHOT docker.elastic.co/beats/elastic-agent-complete:8.8.1-SNAPSHOT-foo
```

## Using Environment variables

Depending on the service you need to use a custom image, you need to set a different environment variable.

Following the example of elastic-agent, it is required to define the `ELASTICSEARCH_IMAGE_REF_OVERRIDE` environment variable before running `elastic-package stack up`:
```
export ELASTIC_AGENT_IMAGE_REF_OVERRIDE="docker.elastic.co/beats/elastic-agent-complete:8.8.2-SNAPSHOT-test"
elastic-package stack up -v -d --version 8.8.2-SNAPSHOT
```

Once the Elastic stack is running, you can check that the docker image used for the elastic-agent image is now the one it was built:

```bash
 $ docker ps --format "{{.Names}} {{.Image}}"
elastic-package-stack-elastic-agent-1 docker.elastic.co/beats/elastic-agent-complete:8.8.2-SNAPSHOT-test
elastic-package-stack-fleet-server-1 docker.elastic.co/beats/elastic-agent-complete:8.8.2-SNAPSHOT-test
elastic-package-stack-kibana-1 docker.elastic.co/kibana/kibana:8.8.2-SNAPSHOT
elastic-package-stack-elasticsearch-1 docker.elastic.co/elasticsearch/elasticsearch:8.8.2-SNAPSHOT
elastic-package-stack-package-registry-1 elastic-package-stack-package-registry
```

## Using the configuration file

These custom version or images can also be defined in the configuration file that is located at
`~/.elastic-package/config.yml`.

```yaml
stack:
    image_ref_overrides: {}
profile:
    current: default
```

To define the same elastic-agent image as in the previous section without any environment variable,
it should be added a map for each Elastic stack version (e.g. 8.8.2-SNAPSHOT) under `image_ref_overrides` key.
Each Elastic stack map would contain the required custom images.

It is important to note that Elastic stack version set in this configuration file must match with the version
set in the `--version` flag.

For instance, the following configuration file sets a custom image for Elastic stack versions `8.8.2-SNAPSHOT` and `8.8.1-SNAPSHOT`:

```yaml
stack:
    image_ref_overrides:
      8.8.2-SNAPSHOT:
         elastic-agent: docker.elastic.co/beats/elastic-agnet-complete:8.8-2-SNAPSHOT-test
         # elasticsearch: <elasticsearch_image>
         # kibana: <kibana_image>
      8.8.1-SNAPSHOT:
         elastic-agent: docker.elastic.co/beats/elastic-agnet-complete:8.8-1-SNAPSHOT-test
         # elasticsearch: <elasticsearch_image>
         # kibana: <kibana_image>
profile:
    current: default
```

If Elastic stack version `8.8.1` is started, then there would not be any custom image spun up.

Some examples with the given configuration file:

```bash
 $ elastic-stack up -v --version 8.8.1-SNAPSHOT
 $ docker ps --format "{{.Names}} {{.Image}}"
elastic-package-stack-elastic-agent-1 docker.elastic.co/beats/elastic-agent-complete:8.8.1-SNAPSHOT-test
elastic-package-stack-fleet-server-1 docker.elastic.co/beats/elastic-agent-complete:8.8.1-SNAPSHOT-test
elastic-package-stack-kibana-1 docker.elastic.co/kibana/kibana:8.8.1-SNAPSHOT
elastic-package-stack-elasticsearch-1 docker.elastic.co/elasticsearch/elasticsearch:8.8.1-SNAPSHOT
elastic-package-stack-package-registry-1 elastic-package-stack-package-registry

 $ elastic-stack up -v --version 8.8.2-SNAPSHOT
 $ docker ps --format "{{.Names}} {{.Image}}"
elastic-package-stack-elastic-agent-1 docker.elastic.co/beats/elastic-agent-complete:8.8.2-SNAPSHOT-test
elastic-package-stack-fleet-server-1 docker.elastic.co/beats/elastic-agent-complete:8.8.2-SNAPSHOT-test
elastic-package-stack-kibana-1 docker.elastic.co/kibana/kibana:8.8.2-SNAPSHOT
elastic-package-stack-elasticsearch-1 docker.elastic.co/elasticsearch/elasticsearch:8.8.2-SNAPSHOT
elastic-package-stack-package-registry-1 elastic-package-stack-package-registry

 $ elastic-stack up -v --version 8.8.1
 $ docker ps --format "{{.Names}} {{.Image}}"
elastic-package-stack-elastic-agent-1 docker.elastic.co/elastic-agent/elastic-agent-complete:8.8.1
elastic-package-stack-fleet-server-1 docker.elastic.co/elastic-agent/elastic-agent-complete:8.8.1
elastic-package-stack-kibana-1 docker.elastic.co/kibana/kibana:8.8.1
elastic-package-stack-elasticsearch-1 docker.elastic.co/elasticsearch/elasticsearch:8.8.1
elastic-package-stack-package-registry-1 elastic-package-stack-package-registry
```
