# Nginx Integration

**Exported fields**

| Field | Description | Type |
|---|---|---|
| @timestamp | Event timestamp. | date |
| data_stream.dataset | Data stream dataset. | constant_keyword |
| data_stream.namespace | Data stream namespace. | constant_keyword |
| data_stream.type | Data stream type. | constant_keyword |
| event.category |  | keyword |
| event.created | Date/time when the event was first read by an agent, or by your pipeline. | date |
| http.request.method |  |  |
| http.request.referrer |  |  |
| http.response.body.bytes |  |  |
| http.response.status_code |  |  |
| http.version |  |  |
| nginx.access.remote_ip_list | An array of remote IP addresses. It is a list because it is common to include, besides the client IP address, IP addresses from headers like `X-Forwarded-For`. Real source IP is restored to `source.ip`. | keyword |
| related.ip | All of the IPs seen on your event. | ip |
| source.address |  |  |
| source.as.number |  |  |
| source.as.organization.name |  |  |
| source.geo.city_name |  |  |
| source.geo.continent_name |  |  |
| source.geo.country_iso_code |  |  |
| source.geo.country_name |  | keyword |
| source.geo.location |  | geo_point |
| source.geo.region_iso_code |  |  |
| source.geo.region_name |  |  |
| source.ip | IP address of the source (IPv4 or IPv6). | ip |
| url.original |  |  |
| user.name |  |  |
| user_agent.device.name | Name of the device. | keyword |
| user_agent.name | Name of the user agent. | keyword |
| user_agent.original | Unparsed user_agent string. | keyword |
| user_agent.os.full |  | keyword |
| user_agent.os.name | Operating system name, without the version. | keyword |
| user_agent.os.version | Operating system version as a raw string. | keyword |
| user_agent.version | Version of the user agent. | keyword |


**Exported fields**

| Field | Description | Type |
|---|---|---|
| @timestamp | Event timestamp. | date |
| data_stream.dataset | Data stream dataset. | constant_keyword |
| data_stream.namespace | Data stream namespace. | constant_keyword |
| data_stream.type | Data stream type. | constant_keyword |
| event.created | Date/time when the event was first read by an agent, or by your pipeline. | date |
| log.level | Original log level of the log event. If the source of the event provides a log level or textual severity, this is the one that goes in `log.level`. If your source doesn't specify one, you may put your event transport's severity here (e.g. Syslog severity). Some examples are `warn`, `err`, `i`, `informational`. | keyword |
| message | For log events the message field contains the log message, optimized for viewing in a log viewer. For structured logs without an original message field, other fields can be concatenated to form a human-readable summary of the event. If multiple messages exist, they can be combined into one message. | text |
| nginx.error.connection_id | Connection identifier. | long |
| process.pid | Process id. | long |
| process.thread.id | Thread ID. | long |


**Exported fields**

| Field | Description | Type |
|---|---|---|
| @timestamp | Event timestamp. | date |
| data_stream.dataset | Data stream dataset. | constant_keyword |
| data_stream.namespace | Data stream namespace. | constant_keyword |
| data_stream.type | Data stream type. | constant_keyword |
| ecs.version |  | keyword |
| nginx.stubstatus.accepts | The total number of accepted client connections. | long |
| nginx.stubstatus.active | The current number of active client connections including Waiting connections. | long |
| nginx.stubstatus.current | The current number of client requests. | long |
| nginx.stubstatus.dropped | The total number of dropped client connections. | long |
| nginx.stubstatus.handled | The total number of handled client connections. | long |
| nginx.stubstatus.hostname | Nginx hostname. | keyword |
| nginx.stubstatus.reading | The current number of connections where Nginx is reading the request header. | long |
| nginx.stubstatus.requests | The total number of client requests. | long |
| nginx.stubstatus.waiting | The current number of idle client connections waiting for a request. | long |
| nginx.stubstatus.writing | The current number of connections where Nginx is writing the response back to the client. | long |
| service.address |  | keyword |
| service.type |  | keyword |
