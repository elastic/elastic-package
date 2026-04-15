# Nginx (composable)

A test integration package that exercises composable integration support by depending on:

- **`filelog_otel`** (input package) — tails Nginx access log files using the OTel filelog receiver.
- **`nginx_otel_input`** (input package) — scrapes Nginx stub_status metrics using the OTel nginx receiver.
- **`nginx_otel`** (content package) — provides dashboards for Nginx OTel data.
