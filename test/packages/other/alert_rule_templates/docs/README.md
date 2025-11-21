# Readme

## Alert rule templates

Alert rule templates provide pre-defined configurations for creating alert rules in Kibana.

For more information, refer to the [Elastic documentation](https://www.elastic.co/docs/reference/fleet/alert-templates#alert-templates).

Alert rule templates require Elastic Stack version 9.2.0 or later.

The following alert rule templates are available:

**[MongoDB Replication] Replica member down**

Alert when a replica set member is down.

Detects any down members in a replica set which can impact availability and replication. When this alert fires, investigate network/connectivity issues, server health, and replica set configuration.

**Metrics:** `mongodb.replstatus.members.down.count`

**Default dimensions:** `[mongodb.replstatus.set_name, service.address]`

**Default threshold:** > 0 (any member down)

**Threshold customization:** For large clusters, you may want to tolerate a single down member depending on voting configuration; adjust thresholds or add runbook checks accordingly.

