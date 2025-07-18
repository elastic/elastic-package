# Nginx Integration

This integration periodically fetches metrics from [Nginx](https://nginx.org/) servers. It can parse access and error
logs created by the HTTP server. 

## Compatibility

The logs were tested with version 1.19.5.
On Windows, the module was tested with Nginx installed from the Chocolatey repository.

## Logs

**Timezone support**

This datasource parses logs that don’t contain timezone information. For these logs, the Elastic Agent reads the local
timezone and uses it when parsing to convert the timestamp to UTC. The timezone to be used for parsing is included
in the event in the `event.timezone` field.

To disable this conversion, the event.timezone field can be removed with the drop_fields processor.

If logs are originated from systems or applications with a different timezone to the local one, the `event.timezone`
field can be overwritten with the original timezone using the add_fields processor.

### Access Logs

Access logs collects the nginx access logs.

An example event for `access` looks as following:

```json
{
    "agent": {
        "hostname": "a73e7856c209",
        "name": "a73e7856c209",
        "id": "3987d2b3-b40a-4aa0-99fc-478f9d7079ea",
        "ephemeral_id": "6d41da1c-5f71-4bd4-b326-a8913bfaa884",
        "type": "filebeat",
        "version": "7.11.0"
    },
    "nginx": {
        "access": {
            "remote_ip_list": [
                "127.0.0.1"
            ]
        }
    },
    "log": {
        "file": {
            "path": "/tmp/service_logs/access.log"
        },
        "offset": 0
    },
    "elastic_agent": {
        "id": "5ca3af72-37c3-48b6-92e8-176d154bb66f",
        "version": "7.11.0",
        "snapshot": true
    },
    "source": {
        "address": "127.0.0.1",
        "ip": "127.0.0.1"
    },
    "url": {
        "original": "/server-status"
    },
    "input": {
        "type": "log"
    },
    "@timestamp": "2020-12-03T11:41:57.000Z",
    "ecs": {
        "version": "1.6.0"
    },
    "related": {
        "ip": [
            "127.0.0.1"
        ]
    },
    "data_stream": {
        "namespace": "ep",
        "type": "logs",
        "dataset": "nginx.access"
    },
    "host": {
        "hostname": "a73e7856c209",
        "os": {
            "kernel": "4.9.184-linuxkit",
            "codename": "Core",
            "name": "CentOS Linux",
            "family": "redhat",
            "version": "7 (Core)",
            "platform": "centos"
        },
        "containerized": true,
        "ip": [
            "192.168.80.6"
        ],
        "name": "a73e7856c209",
        "id": "06c26569966fd125c15acac5d7feffb6",
        "mac": [
            "02:42:c0:a8:50:06"
        ],
        "architecture": "x86_64"
    },
    "http": {
        "request": {
            "method": "get"
        },
        "response": {
            "status_code": 200,
            "body": {
                "bytes": 97
            }
        },
        "version": "1.1"
    },
    "event": {
        "timezone": "+00:00",
        "created": "2020-12-03T11:42:17.116Z",
        "kind": "event",
        "category": [
            "web"
        ],
        "type": [
            "access"
        ],
        "dataset": "nginx.access",
        "outcome": "success"
    },
    "user_agent": {
        "original": "curl/7.64.0",
        "name": "curl",
        "device": {
            "name": "Other"
        },
        "version": "7.64.0"
    }
}
```

**Exported fields**

| Field | Description | Type |
|---|---|---|
| @timestamp | Event timestamp. | date |
| cloud.account.id | The cloud account or organization id used to identify different entities in a multi-tenant environment. Examples: AWS account id, Google Cloud ORG Id, or other unique identifier. | keyword |
| cloud.availability_zone | Availability zone in which this host is running. | keyword |
| cloud.image.id | Image ID for the cloud instance. | keyword |
| cloud.instance.id | Instance ID of the host machine. | keyword |
| cloud.instance.name | Instance name of the host machine. | keyword |
| cloud.machine.type | Machine type of the host machine. | keyword |
| cloud.project.id | Name of the project in Google Cloud. | keyword |
| cloud.provider | Name of the cloud provider. Example values are aws, azure, gcp, or digitalocean. | keyword |
| cloud.region | Region in which this host is running. | keyword |
| container.id | Unique container id. | keyword |
| container.image.name | Name of the image the container was built on. | keyword |
| container.labels | Image labels. | object |
| container.name | Container name. | keyword |
| data_stream.dataset | Data stream dataset. | constant_keyword |
| data_stream.namespace | Data stream namespace. | constant_keyword |
| data_stream.type | Data stream type. | constant_keyword |
| destination.domain | The domain name of the destination system. This value may be a host name, a fully qualified domain name, or another host naming format. The value may derive from the original event or be added from enrichment. | keyword |
| destination.ip | IP address of the destination (IPv4 or IPv6). | ip |
| destination.port | Port of the destination. | long |
| ecs.version | ECS version this event conforms to. `ecs.version` is a required field and must exist in all events. When querying across multiple indices -- which may conform to slightly different ECS versions -- this field lets integrations adjust to the schema version of the events. | keyword |
| event.created | Date/time when the event was first read by an agent, or by your pipeline. | date |
| event.dataset | Event dataset | constant_keyword |
| event.module | Event module | constant_keyword |
| host.architecture | Operating system architecture. | keyword |
| host.containerized | If the host is a container. | boolean |
| host.domain | Name of the domain of which the host is a member. For example, on Windows this could be the host's Active Directory domain or NetBIOS domain name. For Linux this could be the domain of the host's LDAP provider. | keyword |
| host.hostname | Hostname of the host. It normally contains what the `hostname` command returns on the host machine. | keyword |
| host.id | Unique host id. As hostname is not always unique, use values that are meaningful in your environment. Example: The current usage of `beat.name`. | keyword |
| host.ip | Host ip addresses. | ip |
| host.mac | Host mac addresses. | keyword |
| host.name | Name of the host. It can contain what `hostname` returns on Unix systems, the fully qualified domain name, or a name specified by the user. The sender decides which value to use. | keyword |
| host.os.build | OS build information. | keyword |
| host.os.codename | OS codename, if any. | keyword |
| host.os.family | OS family (such as redhat, debian, freebsd, windows). | keyword |
| host.os.kernel | Operating system kernel version as a raw string. | keyword |
| host.os.name | Operating system name, without the version. | keyword |
| host.os.name.text | Multi-field of `host.os.name`. | text |
| host.os.platform | Operating system platform (such centos, ubuntu, windows). | keyword |
| host.os.version | Operating system version as a raw string. | keyword |
| host.type | Type of host. For Cloud providers this can be the machine type like `t2.medium`. If vm, this could be the container, for example, or other information meaningful in your environment. | keyword |
| http.request.method | HTTP request method. The value should retain its casing from the original event. For example, `GET`, `get`, and `GeT` are all considered valid values for this field. | keyword |
| http.request.referrer | Referrer for this HTTP request. | keyword |
| http.response.body.bytes | Size in bytes of the response body. | long |
| http.response.status_code | HTTP response status code. | long |
| http.version | HTTP version. | keyword |
| input.type | Input type | keyword |
| log.file.path | Full path to the log file this event came from, including the file name. It should include the drive letter, when appropriate. If the event wasn't read from a log file, do not populate this field. | keyword |
| log.offset | Log offset | long |
| nginx.access.remote_ip_list | An array of remote IP addresses. It is a list because it is common to include, besides the client IP address, IP addresses from headers like `X-Forwarded-For`. Real source IP is restored to `source.ip`. | array |
| related.ip | All of the IPs seen on your event. | ip |
| source.address | Some event source addresses are defined ambiguously. The event will sometimes list an IP, a domain or a unix socket.  You should always store the raw address in the `.address` field. Then it should be duplicated to `.ip` or `.domain`, depending on which one it is. | keyword |
| source.as.number | Unique number allocated to the autonomous system. The autonomous system number (ASN) uniquely identifies each network on the Internet. | long |
| source.as.organization.name | Organization name. | keyword |
| source.as.organization.name.text | Multi-field of `source.as.organization.name`. | match_only_text |
| source.geo.city_name | City name. | keyword |
| source.geo.continent_name | Name of the continent. | keyword |
| source.geo.country_iso_code | Country ISO code. | keyword |
| source.geo.country_name | Country name. | keyword |
| source.geo.location | Longitude and latitude. | geo_point |
| source.geo.region_iso_code | Region ISO code. | keyword |
| source.geo.region_name | Region name. | keyword |
| source.ip | IP address of the source (IPv4 or IPv6). | ip |
| tags | List of keywords used to tag each event. | keyword |
| url.domain | Domain of the url, such as "www.elastic.co". In some cases a URL may refer to an IP and/or port directly, without a domain name. In this case, the IP address would go to the `domain` field. If the URL contains a literal IPv6 address enclosed by `[` and `]` (IETF RFC 2732), the `[` and `]` characters should also be captured in the `domain` field. | keyword |
| url.extension | The field contains the file extension from the original request url, excluding the leading dot. The file extension is only set if it exists, as not every url has a file extension. The leading period must not be included. For example, the value must be "png", not ".png". Note that when the file name has multiple extensions (example.tar.gz), only the last one should be captured ("gz", not "tar.gz"). | keyword |
| url.fragment | Portion of the url after the `#`, such as "top". The `#` is not part of the fragment. | keyword |
| url.original | Unmodified original url as seen in the event source. Note that in network monitoring, the observed URL may be a full URL, whereas in access logs, the URL is often just represented as a path. This field is meant to represent the URL as it was observed, complete or not. | wildcard |
| url.original.text | Multi-field of `url.original`. | match_only_text |
| url.path | Path of the request, such as "/search". | wildcard |
| url.scheme | Scheme of the request, such as "https". Note: The `:` is not part of the scheme. | keyword |
| user.name | Short name or login of the user. | keyword |
| user.name.text | Multi-field of `user.name`. | match_only_text |
| user_agent.device.name | Name of the device. | keyword |
| user_agent.name | Name of the user agent. | keyword |
| user_agent.original | Unparsed user_agent string. | keyword |
| user_agent.original.text | Multi-field of `user_agent.original`. | match_only_text |
| user_agent.os.full | Operating system name, including the version or code name. | keyword |
| user_agent.os.full.text | Multi-field of `user_agent.os.full`. | match_only_text |
| user_agent.os.name | Operating system name, without the version. | keyword |
| user_agent.os.name.text | Multi-field of `user_agent.os.name`. | match_only_text |
| user_agent.os.version | Operating system version as a raw string. | keyword |
| user_agent.version | Version of the user agent. | keyword |


### Error Logs

Error logs collects the nginx error logs.

An example event for `error` looks as following:

```json
{
    "agent": {
        "hostname": "a73e7856c209",
        "name": "a73e7856c209",
        "id": "3987d2b3-b40a-4aa0-99fc-478f9d7079ea",
        "ephemeral_id": "6d41da1c-5f71-4bd4-b326-a8913bfaa884",
        "type": "filebeat",
        "version": "7.11.0"
    },
    "process": {
        "pid": 1,
        "thread": {
            "id": 1
        }
    },
    "nginx": {
        "error": {}
    },
    "log": {
        "file": {
            "path": "/tmp/service_logs/error.log"
        },
        "offset": 0,
        "level": "warn"
    },
    "elastic_agent": {
        "id": "5ca3af72-37c3-48b6-92e8-176d154bb66f",
        "version": "7.11.0",
        "snapshot": true
    },
    "message": "conflicting server name \"localhost\" on 0.0.0.0:80, ignored",
    "input": {
        "type": "log"
    },
    "@timestamp": "2020-12-03T11:44:39.000Z",
    "ecs": {
        "version": "1.6.0"
    },
    "data_stream": {
        "namespace": "ep",
        "type": "logs",
        "dataset": "nginx.error"
    },
    "host": {
        "hostname": "a73e7856c209",
        "os": {
            "kernel": "4.9.184-linuxkit",
            "codename": "Core",
            "name": "CentOS Linux",
            "family": "redhat",
            "version": "7 (Core)",
            "platform": "centos"
        },
        "containerized": true,
        "ip": [
            "192.168.80.6"
        ],
        "name": "a73e7856c209",
        "id": "06c26569966fd125c15acac5d7feffb6",
        "mac": [
            "02:42:c0:a8:50:06"
        ],
        "architecture": "x86_64"
    },
    "event": {
        "timezone": "+00:00",
        "created": "2020-12-03T11:44:52.803Z",
        "kind": "event",
        "category": [
            "web"
        ],
        "type": [
            "error"
        ],
        "dataset": "nginx.error"
    }
}
```

**Exported fields**

| Field | Description | Type |
|---|---|---|
| @timestamp | Event timestamp. | date |
| cloud.account.id | The cloud account or organization id used to identify different entities in a multi-tenant environment. Examples: AWS account id, Google Cloud ORG Id, or other unique identifier. | keyword |
| cloud.availability_zone | Availability zone in which this host is running. | keyword |
| cloud.image.id | Image ID for the cloud instance. | keyword |
| cloud.instance.id | Instance ID of the host machine. | keyword |
| cloud.instance.name | Instance name of the host machine. | keyword |
| cloud.machine.type | Machine type of the host machine. | keyword |
| cloud.project.id | Name of the project in Google Cloud. | keyword |
| cloud.provider | Name of the cloud provider. Example values are aws, azure, gcp, or digitalocean. | keyword |
| cloud.region | Region in which this host is running. | keyword |
| container.id | Unique container id. | keyword |
| container.image.name | Name of the image the container was built on. | keyword |
| container.labels | Image labels. | object |
| container.name | Container name. | keyword |
| data_stream.dataset | Data stream dataset. | constant_keyword |
| data_stream.namespace | Data stream namespace. | constant_keyword |
| data_stream.type | Data stream type. | constant_keyword |
| ecs.version | ECS version this event conforms to. `ecs.version` is a required field and must exist in all events. When querying across multiple indices -- which may conform to slightly different ECS versions -- this field lets integrations adjust to the schema version of the events. | keyword |
| event.created | Date/time when the event was first read by an agent, or by your pipeline. | date |
| event.dataset | Event dataset | constant_keyword |
| event.module | Event module | constant_keyword |
| host.architecture | Operating system architecture. | keyword |
| host.containerized | If the host is a container. | boolean |
| host.domain | Name of the domain of which the host is a member. For example, on Windows this could be the host's Active Directory domain or NetBIOS domain name. For Linux this could be the domain of the host's LDAP provider. | keyword |
| host.hostname | Hostname of the host. It normally contains what the `hostname` command returns on the host machine. | keyword |
| host.id | Unique host id. As hostname is not always unique, use values that are meaningful in your environment. Example: The current usage of `beat.name`. | keyword |
| host.ip | Host ip addresses. | ip |
| host.mac | Host mac addresses. | keyword |
| host.name | Name of the host. It can contain what `hostname` returns on Unix systems, the fully qualified domain name, or a name specified by the user. The sender decides which value to use. | keyword |
| host.os.build | OS build information. | keyword |
| host.os.codename | OS codename, if any. | keyword |
| host.os.family | OS family (such as redhat, debian, freebsd, windows). | keyword |
| host.os.kernel | Operating system kernel version as a raw string. | keyword |
| host.os.name | Operating system name, without the version. | keyword |
| host.os.name.text | Multi-field of `host.os.name`. | text |
| host.os.platform | Operating system platform (such centos, ubuntu, windows). | keyword |
| host.os.version | Operating system version as a raw string. | keyword |
| host.type | Type of host. For Cloud providers this can be the machine type like `t2.medium`. If vm, this could be the container, for example, or other information meaningful in your environment. | keyword |
| input.type | Input type | keyword |
| log.file.path | Full path to the log file this event came from, including the file name. It should include the drive letter, when appropriate. If the event wasn't read from a log file, do not populate this field. | keyword |
| log.level | Original log level of the log event. If the source of the event provides a log level or textual severity, this is the one that goes in `log.level`. If your source doesn't specify one, you may put your event transport's severity here (e.g. Syslog severity). Some examples are `warn`, `err`, `i`, `informational`. | keyword |
| log.offset | Log offset | long |
| message | For log events the message field contains the log message, optimized for viewing in a log viewer. For structured logs without an original message field, other fields can be concatenated to form a human-readable summary of the event. If multiple messages exist, they can be combined into one message. | match_only_text |
| nginx.error.connection_id | Connection identifier. | long |
| process.pid | Process id. | long |
| process.thread.id | Thread ID. | long |
| tags | List of keywords used to tag each event. | keyword |

