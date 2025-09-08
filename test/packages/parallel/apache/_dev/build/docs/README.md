{{- generatedHeader }}
# Apache Integration for Elastic

## Overview

The Apache integration for Elastic enables collection of access logs, error logs and metrics from [Apache](https://httpd.apache.org/) servers.
This integration facilitates performance monitoring, understanding traffic patterns and gaining security insights from
your Apache servers.

### Compatibility

This integration is compatible with all Apache server versions >= 2.4.16 and >= 2.2.31.

### How it works

Access and error logs data stream read from a logfile. You will configure Apache server to write log files to a location that is readable by elastic-agent, and 
elastic-agent will monitor and ingest events written to these files.

## What data does this integration collect?

The {{.Manifest.Title}} integration collects log messages of the following types:

The Apache HTTP Server integration collects log messages of the following types:

* [Error logs](https://httpd.apache.org/docs/current/logs.html#errorlog)
* [Access logs](https://httpd.apache.org/docs/current/logs.html#accesslog)
* Status metrics

### Supported use cases

The Apache integration for Elastic allows you to collect, parse, and analyze Apache web server logs and metrics within the Elastic Stack.
It centralizes data like access logs, error logs, and performance metrics, transforming unstructured information into a structured, searchable format.
This enables users to monitor performance, troubleshoot issues, gain security insights, and understand website traffic patterns through powerful visualizations in Kibana.
Ultimately, it simplifies the management and analysis of critical data for maintaining a healthy and secure web infrastructure.

## What do I need to use this integration?

Elastic Agent must be installed. For more details, check the Elastic Agent [installation instructions](docs-content://reference/fleet/install-elastic-agents.md). You can install only one Elastic Agent per host.

Elastic Agent is required to stream data from the syslog or log file receiver and ship the data to Elastic, where the events will then be processed via the integration's ingest pipelines.


## How do I deploy this integration?

### Onboard / configure

#### Collect access and error logs

Follow [Apache server instructions](https://httpd.apache.org/docs/2.4/logs.html) to write error or access logs to a location readable by elastic-agent.
Elastic-agent can run on the same system as your Apache server, or the logs can be forwarded to a different system running elastic-agent.

#### Collect metrics

Follow the [Apache server instructions](https://httpd.apache.org/docs/2.4/mod/mod_status.html) to enable the Status module.

#### Enable the integration in Elastic

1. In Kibana navigate to **Management** > **Integrations**.
2. In the search bar, type **Apache HTTP Server**.
3. Select the **Apache HTTP Server** integration and add it.
4. If needed, install Elastic Agent on the systems which will receive error or access log files.
5. Enable and configure only the collection methods which you will use.

    * **To collect logs from Apache instances**, you'll need to add log file path patterns elastic-agent will monitor.

    * **To collect metrics**, you'll need to configure the Apache hosts which will be monitored.

6. Press **Save Integration** to begin collecting logs.

#### Anomaly Detection Configurations

The Apache HTTP server integration also has support of anomaly detection jobs.

These anomaly detection jobs are available in the Machine Learning app in Kibana
when you have data that matches the query specified in the
[manifest](https://github.com/elastic/integrations/blob/main/packages/apache/kibana/ml_module/apache-Logs-ml.json#L11).

##### Apache Access Logs

Find unusual activity in HTTP access logs.

| Job | Description |
|---|---|
| visitor_rate_apache | HTTP Access Logs: Detect unusual visitor rates | 
| status_code_rate_apache | HTTP Access Logs: Detect unusual status code rates |
| source_ip_url_count_apache | HTTP Access Logs: Detect unusual source IPs - high distinct count of URLs |
| source_ip_request_rate_apache | HTTP Access Logs: Detect unusual source IPs - high request rates |
| low_request_rate_apache | HTTP Access Logs: Detect low request rates |

### Validation

<!-- How can the user test whether the integration is working? Including example commands or test files if applicable -->
The "[Logs Apache] Access and error logs" and "[Metrics Apache] Overview" dashboards will show activity for your Apache servers.
After the integration is installed, view these dashboards in Kibana and verify that information for servers is shown.

## Troubleshooting

For help with Elastic ingest tools, check [Common problems](https://www.elastic.co/docs/troubleshoot/ingest/fleet/common-problems).

<!-- Add any vendor specific troubleshooting here.

Are there common issues or “gotchas” for deploying this integration? If so, how can they be resolved?
If applicable, links to the third-party software’s troubleshooting documentation.
-->

## Scaling

For more information on architectures that can be used for scaling this integration, check the [Ingest Architectures](https://www.elastic.co/docs/manage-data/ingest/ingest-reference-architectures) documentation.

## Reference

### ECS field Reference

#### access fields
{{fields "access"}}

### error fields
{{fields "error"}}

### status fields
{{fields "status"}}

### Sample Event

{{event "access"}}

{{event "error"}}

{{event "status"}}

### Inputs used

{{ inputDocs }}

### API usage

These APIs are used with this integration:
* Metrics are collected using the [mod_status](https://httpd.apache.org/docs/current/mod/mod_status.html) Apache module.
