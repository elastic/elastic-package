# HOWTO: Connect the service with the Elastic stack

_This method is a temporary workaround until a finest solution is delivered. Issue: [#190](https://github.com/elastic/elastic-package/issues/190)._

While building or maintaining an integration it's convenient to keep the service connected to the same network as the Elastic
stack instead of linking it temporarily for the execution of system tests. It's especially useful if you're adjusting Kibana
dashboards or want to examine metrics data collected for the service in the Elasticsearch index.

The procedure assumes an existence of two Docker networks - stack network and service network. The goal is to expose
the stack network to the service container. The Elastic Agent will have an access to the service metrics endpoint and its
logs (via mounted volume).

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

If you haven't prepared the service definition yet, it's recommended to do it as it can be also used for service testing.
If you plan to collect also application logs, please add a volume property to the Docker Compose definition:

```yaml
volumes:
  - ${SERVICE_LOGS_DIR}:/usr/local/apache2/logs
```

`SERVICE_LOGS_DIR` is a special environment variable, which points to the local Docker host directory, shared with
the Elastic Agent container.

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