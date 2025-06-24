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

{{event "access"}}

{{fields "access"}}

### Access Docker & TF Logs

Access logs collects the nginx access logs.

{{event "access_docker_tf"}}

{{fields "access_docker_tf"}}


### Error Logs

Error logs collects the nginx error logs.

{{event "error"}}

{{fields "error"}}
