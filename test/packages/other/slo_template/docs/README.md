# Readme

## SLO Templates

SLO templates provide pre-defined configurations for creating SLOs in Kibana.

For more information, refer to the [Elastic documentation](https://www.elastic.co/docs/solutions/observability/incident-management/service-level-objectives-slos).

SLO templates require Elastic Stack version 9.4.0 or later.

**The following SLO templates are available:**

| Name | Description |
|---|---|
| sample_request_availability_99.5_Rolling30Days | This SLO tracks the availability of web server requests, ensuring that 99.5% of HTTP requests receive successful responses (status code \< 400, status_code = 404 or status_code = 429) over a rolling 30-day period to ensure reliable web service delivery. |
| sample_request_latency_p90_500ms_Rolling30Days | This SLO tracks the latency of web server requests, ensuring that 90% of HTTP requests complete within 500 milliseconds over a rolling 30-day period to ensure responsive web service delivery. |
