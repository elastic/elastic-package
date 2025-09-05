# Oracle Integration

This integration is for ingesting Audit Trail logs and fetching performance, tablespace and sysmetric metrics from Oracle Databases.

The integration expects an *.aud audit file that is generated from Oracle Databases by default. If this has been disabled then please see the [Oracle Database Audit Trail Documentation](https://docs.oracle.com/en/database/oracle/oracle-database/19/dbseg/introduction-to-auditing.html#GUID-8D96829C-9151-4FA4-BED9-831D088F12FF).

### Requirements

Connectivity to Oracle can be facilitated in two ways either by using official Oracle libraries or by using a JDBC driver. Facilitation of the connectivity using JDBC is not supported currently with Metricbeat. Connectivity can be facilitated using Oracle libraries and the detailed steps to do the same are mentioned below.

#### Oracle Database Connection Pre-requisites

To get connected with the Oracle Database ORACLE_SID, ORACLE_BASE, ORACLE_HOME environment variables should be set.

For example: Let’s consider Oracle Database 21c installation using RPM manually by following the [Oracle Installation instructions](https://docs.oracle.com/en/database/oracle/oracle-database/21/ladbi/running-rpm-packages-to-install-oracle-database.html). Environment variables should be set as follows:
    `ORACLE_SID=ORCLCDB`
    `ORACLE_BASE=/opt/oracle/oradata`
    `ORACLE_HOME=/opt/oracle/product/21c/dbhome_1`
Also, add `$ORACLE_HOME/bin` to the `PATH` environment variable.

#### Oracle Instant Client

Oracle Instant Client enables development and deployment of applications that connect to Oracle Database. The Instant Client libraries provide the necessary network connectivity and advanced data features to make full use of Oracle Database. If you have OCI Oracle server which comes with these libraries pre-installed, you don't need a separate client installation.

The OCI library install few Client Shared Libraries that must be referenced on the machine where Metricbeat is installed. Please follow the [Oracle Client Installation link](https://docs.oracle.com/en/database/oracle/oracle-database/21/lacli/install-instant-client-using-zip.html#GUID-D3DCB4FB-D3CA-4C25-BE48-3A1FB5A22E84) link for OCI Instant Client set up. The OCI Instant Client is available with the Oracle Universal Installer, RPM file or ZIP file. Download links can be found at the [Oracle Instant Client Download page](https://www.oracle.com/database/technologies/instant-client/downloads.html).

####  Enable Listener

The Oracle listener is a service that runs on the database host and receives requests from Oracle clients. Make sure that [Listener](https://docs.oracle.com/cd/B19306_01/network.102/b14213/lsnrctl.htm) is be running. 
To check if the listener is running or not, run: 

`lsnrctl STATUS`

If the listener is not running, use the command to start:

`lsnrctl START`

Then, Metricbeat can be launched.

*Host Configuration*

The following two types of host configurations are supported:

1. Old style host configuration for backwards compatibility:
    - `hosts: ["user/pass@0.0.0.0:1521/ORCLPDB1.localdomain"]`
    - `hosts: ["user/password@0.0.0.0:1521/ORCLPDB1.localdomain as sysdba"]`

2. DSN host configuration:
    - `hosts: ['user="user" password="pass" connectString="0.0.0.0:1521/ORCLPDB1.localdomain"']`
    - `hosts: ['user="user" password="password" connectString="host:port/service_name" sysdba=true']`


Note: If the password contains the backslash (`\`) character, it must be escaped with a backslash. For example, if the password is `my\_password`, it should be written as `my\\_password`.


## Compatibility

This integration has been tested with Oracle Database 19c, and should work for 18c as well though it has not been tested.


### Memory Metrics 

A Program Global Area (PGA) is a memory region that contains data and control information for a server process. It is nonshared memory created by Oracle Database when a server process is started. Access to the PGA is exclusive to the server process. Metrics concerning Program Global Area (PGA) memory are mentioned below.

**Exported fields**

| Field | Description | Type | Unit | Metric Type |
|---|---|---|---|---|
| @timestamp | Event timestamp. | date |  |  |
| data_stream.dataset | Data stream dataset. | constant_keyword |  |  |
| data_stream.namespace | Data stream namespace. | constant_keyword |  |  |
| data_stream.type | Data stream type. | constant_keyword |  |  |
| ecs.version | ECS version this event conforms to. `ecs.version` is a required field and must exist in all events. When querying across multiple indices -- which may conform to slightly different ECS versions -- this field lets integrations adjust to the schema version of the events. | keyword |  |  |
| event.dataset | Event module | constant_keyword |  |  |
| event.module | Event module | constant_keyword |  |  |
| host.ip | Host ip addresses. | ip |  |  |
| oracle.memory.pga.aggregate_auto_target | Amount of PGA memory the Oracle Database can use for work areas running in automatic mode. | double | byte | gauge |
| oracle.memory.pga.aggregate_target_parameter | Current value of the PGA_AGGREGATE_TARGET initialization parameter. If this parameter is not set, then its value is 0 and automatic management of PGA memory is disabled. | double | byte | gauge |
| oracle.memory.pga.cache_hit_pct | A metric computed by the Oracle Database to reflect the performance of the PGA memory component, cumulative since instance startup. | double | percent | gauge |
| oracle.memory.pga.global_memory_bound | Maximum size of a work area executed in automatic mode. | double | byte | gauge |
| oracle.memory.pga.maximum_allocated | Maximum number of bytes of PGA memory allocated at one time since instance startup. | double | byte | gauge |
| oracle.memory.pga.total_allocated | Current amount of PGA memory allocated by the instance. | double | byte | gauge |
| oracle.memory.pga.total_freeable_memory | Number of bytes of PGA memory in all processes that could be freed back to the operating system. | double | byte | gauge |
| oracle.memory.pga.total_inuse | Indicates how much PGA memory is currently consumed by work areas. This number can be used to determine how much memory is consumed by other consumers of the PGA memory (for example, PL/SQL or Java). | double | byte | gauge |
| oracle.memory.pga.total_used_for_auto_workareas | Indicates how much PGA memory is currently consumed by work areas running under the automatic memory management mode. This number can be used to determine how much memory is consumed by other consumers of the PGA memory (for example, PL/SQL or Java). | double | byte | gauge |
| oracle.memory.sga.free_memory | Amount of free memory in the Shared pool. | double | byte | gauge |
| oracle.memory.sga.total_memory | Amount of total memory in the Shared pool. | double | byte | gauge |
| service.address | Address where data about this service was collected from. This should be a URI, network address (ipv4:port or [ipv6]:port) or a resource path (sockets). | keyword |  |  |
| service.type | The type of the service data is collected from. The type can be used to group and correlate logs and metrics from one service type. Example: If logs or metrics are collected from Elasticsearch, `service.type` would be `elasticsearch`. | keyword |  |  |


An example event for `memory` looks as following:

```json
{
    "@timestamp": "2022-10-07T13:39:29.922Z",
    "agent": {
        "ephemeral_id": "8dd84084-f02d-40df-8642-ad22035bbf78",
        "id": "f5f65ff4-151c-4b09-876d-a46acf38f06d",
        "name": "docker-custom-agent",
        "type": "metricbeat",
        "version": "8.4.0"
    },
    "cloud": {
        "account": {
            "id": "elastic-obs-integrations-dev"
        },
        "availability_zone": "asia-south1-c",
        "instance": {
            "id": "3010911784348669868",
            "name": "service-integration-dev-idc-01"
        },
        "machine": {
            "type": "n1-standard-8"
        },
        "project": {
            "id": "elastic-obs-integrations-dev"
        },
        "provider": "gcp",
        "service": {
            "name": "GCE"
        }
    },
    "data_stream": {
        "dataset": "oracle.memory",
        "namespace": "ep",
        "type": "metrics"
    },
    "ecs": {
        "version": "8.0.0"
    },
    "elastic_agent": {
        "id": "f5f65ff4-151c-4b09-876d-a46acf38f06d",
        "snapshot": false,
        "version": "8.4.0"
    },
    "event": {
        "agent_id_status": "verified",
        "dataset": "oracle.memory",
        "duration": 103851536,
        "ingested": "2022-10-07T13:39:31Z",
        "module": "sql"
    },
    "host": {
        "architecture": "x86_64",
        "containerized": true,
        "hostname": "docker-custom-agent",
        "id": "5016511f0829451ea244f458eebf2212",
        "ip": [
            "172.25.0.2",
            "172.23.0.5"
        ],
        "mac": [
            "02:42:ac:17:00:05",
            "02:42:ac:19:00:02"
        ],
        "name": "docker-custom-agent",
        "os": {
            "codename": "focal",
            "family": "debian",
            "kernel": "5.4.0-1078-gcp",
            "name": "Ubuntu",
            "platform": "ubuntu",
            "type": "linux",
            "version": "20.04.4 LTS (Focal Fossa)"
        }
    },
    "metricset": {
        "name": "query",
        "period": 60000
    },
    "oracle": {
        "memory": {
            "pga": {
                "aggregate_auto_target": 569124864,
                "aggregate_target_parameter": 805306368,
                "cache_hit_pct": 100,
                "global_memory_bound": 104857600,
                "maximum_allocated": 334553088,
                "total_allocated": 200572928,
                "total_freeable_memory": 34930688,
                "total_inuse": 147128320,
                "total_used_for_auto_workareas": 0
            },
            "sga": {
                "free_memory": 30123968,
                "total_memory": 335544320
            }
        }
    },
    "service": {
        "address": "elastic-package-service_oracle_1:1521",
        "type": "sql"
    }
}
```


