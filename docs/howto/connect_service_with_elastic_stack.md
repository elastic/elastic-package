# HOWTO: Connect the service with the Elastic stack

_This method is a temporary workaround until a finest solution is delivered. Issue: [#190](https://github.com/elastic/elastic-package/issues/190)._

While building or maintaining an integration it's convenient to keep the service connected to the same network as the Elastic
stack instead of linking it temporarily for the execution of system tests. It's especially useful if you're adjusting Kibana
dashboards or want to examine metrics data collected for the service in the Elasticsearch index.

The procedure assumes an existence of two Docker networks - stack network and service network. The goal is to expose
the stack network to the service container. The Elastic Agent will have an access to the service metrics endpoint and its
logs (via mounted volume).

## Prerequisites

### Service Docker Compose definition

The service can be defined using Docker files (YML format), which describe Docker images required for the service to be up.
The running service produces metrics and logs, which later on can be collected by the Elastic Agent. Such instance is
a good candidate for system tests or manual testing.

#### Support for logs collection

As the Elastic Agent is running in a separate container, there is shared volume for logs required. The volume is shared
between the Elastic Agent and the service container.

In the Elastic Agent container logs are exposed under `/tmp/service_logs`:

```yaml
volumes:
- type: bind
  source: ../tmp/service_logs/
  target: /tmp/service_logs/
```

We need to define a similar volume in the service `docker-compose.yml` file:

```yaml
volumes:
- ${SERVICE_LOGS_DIR}:/usr/local/apache2/logs
```

`SERVICE_LOGS_DIR` is a special environment variable, which points to the local Docker host directory, shared with
the Elastic Agent container.

Review the [docker-compose.yml](https://github.com/elastic/elastic-package/blob/master/test/packages/apache/_dev/deploy/docker/docker-compose.yml#L11)
file for Apache to see a real example.

## User guide (based on the Apache integration)

1. If you haven't done it already, please boot up the Elastic stack:

```bash
elastic-package stack up -d -v
```

2. Boot up the Apache service (using Docker Compose definitions):

```bash
cd packages/apache/_dev/deploy/docker
SERVICE_LOGS_DIR=/Users/JohnDoe/.elastic-package/tmp/service_logs docker-compose -p elastic-package-service up -d
```

The command should bring up the Docker containers specified in the Docker Compose definitions.

3. Connect the service container with the Elastic stack Docker network:

```bash
docker network connect elastic-package-stack_default elastic-package-service_apache_1
```

The service container should be reachable from the Elastic stack network. Let's verify it!

4. Verify the connectivity between the Elastic Agent container and the service one.

Jump into the Elastic Agent container:

```bash
docker exec -it <elastic agent container ID> /bin/bash
```

Check connection with the service container:

```bash
curl http://elastic-package-service_apache_1:80/server-status -I
HTTP/1.1 200 OK
Date: Wed, 25 Nov 2020 11:49:58 GMT
Server: Apache/2.4.20 (Unix)
Content-Type: text/html; charset=ISO-8859-1
```

Check presence of application log files:

```bash
ls /tmp/service_logs
access.log  error.log  httpd.pid
```

5. Define the service endpoint in the Kibana Fleet UI.

When you need to specify the service address, keep in mind that the Elastic stack and the service operate in Docker networks,
so they don't communicate over the `localhost`, but use dedicated domains instead, e.g.: `elastic-package-service_apache_1:80`.

6. Define the service's log path in the Kibana Fleet UI.

When you need to specify the log path, use the path as it is mounted in Agent's container, for example `/tmp/service_logs/access.log`.

## Advanced

In general it's advised not to modify the Docker Compose files for the Elastic stack.

In case of more advanced use cases or workaorounds, you can try to temporarily modify Docker Compose files.
Files are available in: `~/.elastic-package/stack/`. The `snapshot.yml` file contains the main definition of the stack
and its services (Elasticsearch, Kibana, Package Registry).

Please keep in mind that the `~/.elastic-package` directory is cleaned on the builder tool update, so there is a risk to
lose your modification.
