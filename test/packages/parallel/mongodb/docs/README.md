# MongoDB Integration

This integration is used to fetch logs and metrics from [MongoDB](https://www.mongodb.com/).

## Compatibility

The `log` dataset is tested with logs from versions v3.2.11 and v4.4.4 in
plaintext and json formats.
The `collstats`, `dbstats`, `metrics`, `replstatus` and `status` datasets are 
tested with MongoDB 3.4 and 3.0 and are expected to work with all versions >= 2.8.

## MongoDB Privileges
In order to use the metrics datasets, the MongoDB user specified in the package
configuration needs to have certain [privileges](https://docs.mongodb.com/manual/core/authorization/#privileges).

We recommend using the [clusterMonitor](https://docs.mongodb.com/manual/reference/built-in-roles/#clusterMonitor) 
role to cover all the necessary privileges.

You can use the following command in Mongo shell to create the privileged user
(make sure you are using the `admin` db by using `db` command in Mongo shell).

```
db.createUser(
    {
        user: "beats",
        pwd: "pass",
        roles: ["clusterMonitor"]
    }
)
```

You can use the following command in Mongo shell to grant the role to an 
existing user (make sure you are using the `admin` db by using `db` command in 
Mongo shell).

```
db.grantRolesToUser("user", ["clusterMonitor"])
```

## Metrics

### status

The `status` returns a document that provides an overview of the database's state.

It requires the following privileges, which is covered by the [clusterMonitor](https://docs.mongodb.com/manual/reference/built-in-roles/#clusterMonitor) role:

* [serverStatus](https://docs.mongodb.com/manual/reference/privilege-actions/#serverStatus) 
action on [cluster resource](https://docs.mongodb.com/manual/reference/resource-document/#cluster-resource)

An example event for `status` looks as following:

```json
{
    "@timestamp": "2023-06-29T18:00:46.865Z",
    "agent": {
        "ephemeral_id": "3a77a36c-6a5e-4e95-972d-c7e78c813e03",
        "id": "3bcbc378-6f9d-4e10-9bf2-b99c50493aa5",
        "name": "docker-fleet-agent",
        "type": "metricbeat",
        "version": "8.8.0"
    },
    "data_stream": {
        "dataset": "mongodb.status",
        "namespace": "ep",
        "type": "metrics"
    },
    "ecs": {
        "version": "8.0.0"
    },
    "elastic_agent": {
        "id": "3bcbc378-6f9d-4e10-9bf2-b99c50493aa5",
        "snapshot": false,
        "version": "8.8.0"
    },
    "event": {
        "agent_id_status": "verified",
        "dataset": "mongodb.status",
        "duration": 3735431,
        "ingested": "2023-06-29T18:00:47Z",
        "module": "mongodb"
    },
    "host": {
        "architecture": "x86_64",
        "containerized": false,
        "hostname": "docker-fleet-agent",
        "id": "e8978f2086c14e13b7a0af9ed0011d19",
        "ip": "192.168.160.7",
        "mac": "02-42-C0-A8-A0-07",
        "name": "docker-fleet-agent",
        "os": {
            "codename": "focal",
            "family": "debian",
            "kernel": "5.19.0-43-generic",
            "name": "Ubuntu",
            "platform": "ubuntu",
            "type": "linux",
            "version": "20.04.6 LTS (Focal Fossa)"
        }
    },
    "metricset": {
        "name": "status",
        "period": 10000
    },
    "mongodb": {
        "status": {
            "asserts": {
                "msg": 0,
                "regular": 0,
                "rollovers": 0,
                "user": 118,
                "warning": 0
            },
            "connections": {
                "available": 838857,
                "current": 3,
                "total_created": 18
            },
            "extra_info": {
                "page_faults": 0
            },
            "global_lock": {
                "active_clients": {
                    "readers": 0,
                    "total": 0,
                    "writers": 0
                },
                "current_queue": {
                    "readers": 0,
                    "total": 0,
                    "writers": 0
                },
                "total_time": {
                    "us": 17359000
                }
            },
            "local_time": "2023-06-29T18:00:46.866Z",
            "locks": {
                "collection": {
                    "acquire": {
                        "count": {
                            "W": 2,
                            "r": 13,
                            "w": 5
                        }
                    }
                },
                "database": {
                    "acquire": {
                        "count": {
                            "W": 1,
                            "r": 13,
                            "w": 7
                        }
                    }
                },
                "global": {
                    "acquire": {
                        "count": {
                            "W": 5,
                            "r": 65,
                            "w": 8
                        }
                    },
                    "wait": {
                        "count": {
                            "r": 1
                        },
                        "us": {
                            "r": 16
                        }
                    }
                }
            },
            "memory": {
                "bits": 64,
                "resident": {
                    "mb": 113
                },
                "virtual": {
                    "mb": 1510
                }
            },
            "network": {
                "in": {
                    "bytes": 14411
                },
                "out": {
                    "bytes": 937002
                },
                "requests": 140
            },
            "ops": {
                "counters": {
                    "command": 143,
                    "delete": 0,
                    "getmore": 0,
                    "insert": 0,
                    "query": 0,
                    "update": 0
                },
                "latencies": {
                    "commands": {
                        "count": 138,
                        "latency": 15242
                    },
                    "reads": {
                        "count": 0,
                        "latency": 0
                    },
                    "writes": {
                        "count": 0,
                        "latency": 0
                    }
                },
                "replicated": {
                    "command": 0,
                    "delete": 0,
                    "getmore": 0,
                    "insert": 0,
                    "query": 0,
                    "update": 0
                }
            },
            "storage_engine": {
                "name": "wiredTiger"
            },
            "uptime": {
                "ms": 17352
            },
            "wired_tiger": {
                "cache": {
                    "dirty": {
                        "bytes": 30796
                    },
                    "maximum": {
                        "bytes": 16148070400
                    },
                    "pages": {
                        "evicted": 0,
                        "read": 0,
                        "write": 0
                    },
                    "used": {
                        "bytes": 33192
                    }
                },
                "concurrent_transactions": {
                    "read": {
                        "available": 128,
                        "out": 0,
                        "total_tickets": 128
                    },
                    "write": {
                        "available": 128,
                        "out": 0,
                        "total_tickets": 128
                    }
                },
                "log": {
                    "flushes": 170,
                    "max_file_size": {
                        "bytes": 104857600
                    },
                    "scans": 0,
                    "size": {
                        "bytes": 33554432
                    },
                    "syncs": 14,
                    "write": {
                        "bytes": 21504
                    },
                    "writes": 64
                }
            }
        }
    },
    "process": {
        "name": "mongod"
    },
    "service": {
        "address": "mongodb://elastic-package-service-mongodb-1",
        "type": "mongodb",
        "version": "5.0.18"
    }
}
```

The fields reported are:

**Exported fields**

| Field | Description | Type | Metric Type |
|---|---|---|---|
| @timestamp | Event timestamp. | date |  |
| agent.id |  | keyword |  |
| cloud.account.id | The cloud account or organization id used to identify different entities in a multi-tenant environment. Examples: AWS account id, Google Cloud ORG Id, or other unique identifier. | keyword |  |
| cloud.availability_zone | Availability zone in which this host is running. | keyword |  |
| cloud.image.id | Image ID for the cloud instance. | keyword |  |
| cloud.instance.id | Instance ID of the host machine. | keyword |  |
| cloud.instance.name | Instance name of the host machine. | keyword |  |
| cloud.machine.type | Machine type of the host machine. | keyword |  |
| cloud.project.id | Name of the project in Google Cloud. | keyword |  |
| cloud.provider | Name of the cloud provider. Example values are aws, azure, gcp, or digitalocean. | keyword |  |
| cloud.region | Region in which this host is running. | keyword |  |
| container.id | Unique container id. | keyword |  |
| container.image.name | Name of the image the container was built on. | keyword |  |
| container.labels | Image labels. | object |  |
| container.name | Container name. | keyword |  |
| data_stream.dataset | Data stream dataset. | constant_keyword |  |
| data_stream.namespace | Data stream namespace. | constant_keyword |  |
| data_stream.type | Data stream type. | constant_keyword |  |
| ecs.version | ECS version this event conforms to. `ecs.version` is a required field and must exist in all events. When querying across multiple indices -- which may conform to slightly different ECS versions -- this field lets integrations adjust to the schema version of the events. | keyword |  |
| event.dataset | Event dataset | constant_keyword |  |
| event.module | Event module | constant_keyword |  |
| host.architecture | Operating system architecture. | keyword |  |
| host.containerized | If the host is a container. | boolean |  |
| host.domain | Name of the domain of which the host is a member. For example, on Windows this could be the host's Active Directory domain or NetBIOS domain name. For Linux this could be the domain of the host's LDAP provider. | keyword |  |
| host.hostname | Hostname of the host. It normally contains what the `hostname` command returns on the host machine. | keyword |  |
| host.id | Unique host id. As hostname is not always unique, use values that are meaningful in your environment. Example: The current usage of `beat.name`. | keyword |  |
| host.ip | Host ip addresses. | ip |  |
| host.mac | Host mac addresses. | keyword |  |
| host.name | Name of the host. It can contain what `hostname` returns on Unix systems, the fully qualified domain name, or a name specified by the user. The sender decides which value to use. | keyword |  |
| host.os.build | OS build information. | keyword |  |
| host.os.codename | OS codename, if any. | keyword |  |
| host.os.family | OS family (such as redhat, debian, freebsd, windows). | keyword |  |
| host.os.kernel | Operating system kernel version as a raw string. | keyword |  |
| host.os.name | Operating system name, without the version. | keyword |  |
| host.os.name.text | Multi-field of `host.os.name`. | text |  |
| host.os.platform | Operating system platform (such centos, ubuntu, windows). | keyword |  |
| host.os.version | Operating system version as a raw string. | keyword |  |
| host.type | Type of host. For Cloud providers this can be the machine type like `t2.medium`. If vm, this could be the container, for example, or other information meaningful in your environment. | keyword |  |
| mongodb.status.asserts.msg | Number of msg assertions produced by the server. | long | counter |
| mongodb.status.asserts.regular | Number of regular assertions produced by the server. | long | counter |
| mongodb.status.asserts.rollovers | Number of rollovers assertions produced by the server. | long | counter |
| mongodb.status.asserts.user | Number of user assertions produced by the server. | long | counter |
| mongodb.status.asserts.warning | Number of warning assertions produced by the server. | long | counter |
| mongodb.status.background_flushing.average.ms | The average time spent flushing to disk per flush event. | long | gauge |
| mongodb.status.background_flushing.flushes | A counter that collects the number of times the database has flushed all writes to disk. | long | counter |
| mongodb.status.background_flushing.last.ms | The amount of time, in milliseconds, that the last flush operation took to complete. | long | gauge |
| mongodb.status.background_flushing.last_finished | A timestamp of the last completed flush operation. | date |  |
| mongodb.status.background_flushing.total.ms | The total number of milliseconds (ms) that the mongod processes have spent writing (i.e. flushing) data to disk. Because this is an absolute value, consider the value of `flushes` and `average_ms` to provide better context for this datum. | long | gauge |
| mongodb.status.connections.available | The number of unused available incoming connections the database can provide. | long | gauge |
| mongodb.status.connections.current | The number of connections to the database server from clients. This number includes the current shell session. Consider the value of `available` to add more context to this datum. | long | gauge |
| mongodb.status.connections.total_created | A count of all incoming connections created to the server. This number includes connections that have since closed. | long | counter |
| mongodb.status.extra_info.heap_usage.bytes | The total size in bytes of heap space used by the database process. Only available on Unix/Linux. | long | gauge |
| mongodb.status.extra_info.page_faults | The total number of page faults that require disk operations. Page faults refer to operations that require the database server to access data that isn't available in active memory. | long | counter |
| mongodb.status.global_lock.active_clients.readers | The number of the active client connections performing read operations. | long | gauge |
| mongodb.status.global_lock.active_clients.total | Total number of the active client connections performing read or write operations. | long | gauge |
| mongodb.status.global_lock.active_clients.writers | The number of the active client connections performing write operations. | long | gauge |
| mongodb.status.global_lock.current_queue.readers | The number of operations that are currently queued and waiting for the read lock. | long | gauge |
| mongodb.status.global_lock.current_queue.total | The total number of operations queued waiting for the lock (i.e., the sum of current_queue.readers and current_queue.writers). | long | gauge |
| mongodb.status.global_lock.current_queue.writers | The number of operations that are currently queued and waiting for the write lock. | long | gauge |
| mongodb.status.global_lock.total_time.us | The time, in microseconds, since the database last started and created the globalLock. This is roughly equivalent to total server uptime. | long | gauge |
| mongodb.status.journaling.commits | The number of transactions written to the journal during the last journal group commit interval. | long | counter |
| mongodb.status.journaling.commits_in_write_lock | Count of the commits that occurred while a write lock was held. Commits in a write lock indicate a MongoDB node under a heavy write load and call for further diagnosis. | long | counter |
| mongodb.status.journaling.compression | The compression ratio of the data written to the journal. | long | gauge |
| mongodb.status.journaling.early_commits | The number of times MongoDB requested a commit before the scheduled journal group commit interval. | long | counter |
| mongodb.status.journaling.journaled.mb | The amount of data in megabytes (MB) written to journal during the last journal group commit interval. | long | gauge |
| mongodb.status.journaling.times.commits.ms | The amount of time spent for commits. | long | gauge |
| mongodb.status.journaling.times.commits_in_write_lock.ms | The amount of time spent for commits that occurred while a write lock was held. | long | gauge |
| mongodb.status.journaling.times.dt.ms | The amount of time over which MongoDB collected the times data. Use this field to provide context to the other times field values. | long | gauge |
| mongodb.status.journaling.times.prep_log_buffer.ms | The amount of time spent preparing to write to the journal. Smaller values indicate better journal performance. | long | gauge |
| mongodb.status.journaling.times.remap_private_view.ms | The amount of time spent remapping copy-on-write memory mapped views. Smaller values indicate better journal performance. | long | gauge |
| mongodb.status.journaling.times.write_to_data_files.ms | The amount of time spent writing to data files after journaling. File system speeds and device interfaces can affect performance. | long | gauge |
| mongodb.status.journaling.times.write_to_journal.ms | The amount of time spent actually writing to the journal. File system speeds and device interfaces can affect performance. | long | gauge |
| mongodb.status.journaling.write_to_data_files.mb | The amount of data in megabytes (MB) written from journal to the data files during the last journal group commit interval. | long | gauge |
| mongodb.status.local_time | Local time as reported by the MongoDB instance. | date |  |
| mongodb.status.locks.collection.acquire.count.R |  | long | counter |
| mongodb.status.locks.collection.acquire.count.W |  | long | counter |
| mongodb.status.locks.collection.acquire.count.r |  | long | counter |
| mongodb.status.locks.collection.acquire.count.w |  | long | counter |
| mongodb.status.locks.collection.deadlock.count.R |  | long | counter |
| mongodb.status.locks.collection.deadlock.count.W |  | long | counter |
| mongodb.status.locks.collection.deadlock.count.r |  | long | counter |
| mongodb.status.locks.collection.deadlock.count.w |  | long | counter |
| mongodb.status.locks.collection.wait.count.R |  | long | counter |
| mongodb.status.locks.collection.wait.count.W |  | long | counter |
| mongodb.status.locks.collection.wait.count.r |  | long | counter |
| mongodb.status.locks.collection.wait.count.w |  | long | counter |
| mongodb.status.locks.collection.wait.us.R |  | long | gauge |
| mongodb.status.locks.collection.wait.us.W |  | long | gauge |
| mongodb.status.locks.collection.wait.us.r |  | long | gauge |
| mongodb.status.locks.collection.wait.us.w |  | long | gauge |
| mongodb.status.locks.database.acquire.count.R |  | long | counter |
| mongodb.status.locks.database.acquire.count.W |  | long | counter |
| mongodb.status.locks.database.acquire.count.r |  | long | counter |
| mongodb.status.locks.database.acquire.count.w |  | long | counter |
| mongodb.status.locks.database.deadlock.count.R |  | long | counter |
| mongodb.status.locks.database.deadlock.count.W |  | long | counter |
| mongodb.status.locks.database.deadlock.count.r |  | long | counter |
| mongodb.status.locks.database.deadlock.count.w |  | long | counter |
| mongodb.status.locks.database.wait.count.R |  | long | counter |
| mongodb.status.locks.database.wait.count.W |  | long | counter |
| mongodb.status.locks.database.wait.count.r |  | long | counter |
| mongodb.status.locks.database.wait.count.w |  | long | counter |
| mongodb.status.locks.database.wait.us.R |  | long | gauge |
| mongodb.status.locks.database.wait.us.W |  | long | gauge |
| mongodb.status.locks.database.wait.us.r |  | long | gauge |
| mongodb.status.locks.database.wait.us.w |  | long | gauge |
| mongodb.status.locks.global.acquire.count.R |  | long | counter |
| mongodb.status.locks.global.acquire.count.W |  | long | counter |
| mongodb.status.locks.global.acquire.count.r |  | long | counter |
| mongodb.status.locks.global.acquire.count.w |  | long | counter |
| mongodb.status.locks.global.deadlock.count.R |  | long | counter |
| mongodb.status.locks.global.deadlock.count.W |  | long | counter |
| mongodb.status.locks.global.deadlock.count.r |  | long | counter |
| mongodb.status.locks.global.deadlock.count.w |  | long | counter |
| mongodb.status.locks.global.wait.count.R |  | long | counter |
| mongodb.status.locks.global.wait.count.W |  | long | counter |
| mongodb.status.locks.global.wait.count.r |  | long | counter |
| mongodb.status.locks.global.wait.count.w |  | long | counter |
| mongodb.status.locks.global.wait.us.R |  | long | gauge |
| mongodb.status.locks.global.wait.us.W |  | long | gauge |
| mongodb.status.locks.global.wait.us.r |  | long | gauge |
| mongodb.status.locks.global.wait.us.w |  | long | gauge |
| mongodb.status.locks.meta_data.acquire.count.R |  | long | counter |
| mongodb.status.locks.meta_data.acquire.count.W |  | long | counter |
| mongodb.status.locks.meta_data.acquire.count.r |  | long | counter |
| mongodb.status.locks.meta_data.acquire.count.w |  | long | counter |
| mongodb.status.locks.meta_data.deadlock.count.R |  | long | counter |
| mongodb.status.locks.meta_data.deadlock.count.W |  | long | counter |
| mongodb.status.locks.meta_data.deadlock.count.r |  | long | counter |
| mongodb.status.locks.meta_data.deadlock.count.w |  | long | counter |
| mongodb.status.locks.meta_data.wait.count.R |  | long | counter |
| mongodb.status.locks.meta_data.wait.count.W |  | long | counter |
| mongodb.status.locks.meta_data.wait.count.r |  | long | counter |
| mongodb.status.locks.meta_data.wait.count.w |  | long | counter |
| mongodb.status.locks.meta_data.wait.us.R |  | long | gauge |
| mongodb.status.locks.meta_data.wait.us.W |  | long | gauge |
| mongodb.status.locks.meta_data.wait.us.r |  | long | gauge |
| mongodb.status.locks.meta_data.wait.us.w |  | long | gauge |
| mongodb.status.locks.oplog.acquire.count.R |  | long | counter |
| mongodb.status.locks.oplog.acquire.count.W |  | long | counter |
| mongodb.status.locks.oplog.acquire.count.r |  | long | counter |
| mongodb.status.locks.oplog.acquire.count.w |  | long | counter |
| mongodb.status.locks.oplog.deadlock.count.R |  | long | counter |
| mongodb.status.locks.oplog.deadlock.count.W |  | long | counter |
| mongodb.status.locks.oplog.deadlock.count.r |  | long | counter |
| mongodb.status.locks.oplog.deadlock.count.w |  | long | counter |
| mongodb.status.locks.oplog.wait.count.R |  | long | counter |
| mongodb.status.locks.oplog.wait.count.W |  | long | counter |
| mongodb.status.locks.oplog.wait.count.r |  | long | counter |
| mongodb.status.locks.oplog.wait.count.w |  | long | counter |
| mongodb.status.locks.oplog.wait.us.R |  | long | gauge |
| mongodb.status.locks.oplog.wait.us.W |  | long | gauge |
| mongodb.status.locks.oplog.wait.us.r |  | long | gauge |
| mongodb.status.locks.oplog.wait.us.w |  | long | gauge |
| mongodb.status.memory.bits | Either 64 or 32, depending on which target architecture was specified during the mongod compilation process. | long |  |
| mongodb.status.memory.mapped.mb | The amount of mapped memory, in megabytes (MB), used by the database. Because MongoDB uses memory-mapped files, this value is likely to be to be roughly equivalent to the total size of your database or databases. | long | gauge |
| mongodb.status.memory.mapped_with_journal.mb | The amount of mapped memory, in megabytes (MB), including the memory used for journaling. | long | gauge |
| mongodb.status.memory.resident.mb | The amount of RAM, in megabytes (MB), currently used by the database process. | long | gauge |
| mongodb.status.memory.virtual.mb | The amount, in megabytes (MB), of virtual memory used by the mongod process. | long | gauge |
| mongodb.status.network.in.bytes | The amount of network traffic, in bytes, received by this database. | long | gauge |
| mongodb.status.network.out.bytes | The amount of network traffic, in bytes, sent from this database. | long | gauge |
| mongodb.status.network.requests | The total number of requests received by the server. | long | counter |
| mongodb.status.ops.counters.command | The total number of commands issued to the database since the mongod instance last started. | long | counter |
| mongodb.status.ops.counters.delete | The total number of delete operations received since the mongod instance last started. | long | counter |
| mongodb.status.ops.counters.getmore | The total number of getmore operations received since the mongod instance last started. | long | counter |
| mongodb.status.ops.counters.insert | The total number of insert operations received since the mongod instance last started. | long | counter |
| mongodb.status.ops.counters.query | The total number of queries received since the mongod instance last started. | long | counter |
| mongodb.status.ops.counters.update | The total number of update operations received since the mongod instance last started. | long | counter |
| mongodb.status.ops.latencies.commands.count | Total number of commands performed on the collection since startup. | long | counter |
| mongodb.status.ops.latencies.commands.latency | Total combined latency in microseconds. | long | gauge |
| mongodb.status.ops.latencies.reads.count | Total number of read operations performed on the collection since startup. | long | counter |
| mongodb.status.ops.latencies.reads.latency | Total combined latency in microseconds. | long | gauge |
| mongodb.status.ops.latencies.writes.count | Total number of write operations performed on the collection since startup. | long | counter |
| mongodb.status.ops.latencies.writes.latency | Total combined latency in microseconds. | long | gauge |
| mongodb.status.ops.replicated.command | The total number of replicated commands issued to the database since the mongod instance last started. | long | counter |
| mongodb.status.ops.replicated.delete | The total number of replicated delete operations received since the mongod instance last started. | long | counter |
| mongodb.status.ops.replicated.getmore | The total number of replicated getmore operations received since the mongod instance last started. | long | counter |
| mongodb.status.ops.replicated.insert | The total number of replicated insert operations received since the mongod instance last started. | long | counter |
| mongodb.status.ops.replicated.query | The total number of replicated queries received since the mongod instance last started. | long | counter |
| mongodb.status.ops.replicated.update | The total number of replicated update operations received since the mongod instance last started. | long | counter |
| mongodb.status.storage_engine.name | A string that represents the name of the current storage engine. | keyword |  |
| mongodb.status.uptime.ms | Instance uptime in milliseconds. | long | gauge |
| mongodb.status.wired_tiger.cache.dirty.bytes | Size in bytes of the dirty data in the cache. | long | gauge |
| mongodb.status.wired_tiger.cache.maximum.bytes | Maximum cache size. | long | gauge |
| mongodb.status.wired_tiger.cache.pages.evicted | Number of pages evicted from the cache. | long | counter |
| mongodb.status.wired_tiger.cache.pages.read | Number of pages read into the cache. | long | counter |
| mongodb.status.wired_tiger.cache.pages.write | Number of pages written from the cache. | long | counter |
| mongodb.status.wired_tiger.cache.used.bytes | Size in byte of the data currently in cache. | long | gauge |
| mongodb.status.wired_tiger.concurrent_transactions.read.available | Number of concurrent read tickets available. | long | gauge |
| mongodb.status.wired_tiger.concurrent_transactions.read.out | Number of concurrent read transaction in progress. | long | gauge |
| mongodb.status.wired_tiger.concurrent_transactions.read.total_tickets | Number of total read tickets. | long | gauge |
| mongodb.status.wired_tiger.concurrent_transactions.write.available | Number of concurrent write tickets available. | long | gauge |
| mongodb.status.wired_tiger.concurrent_transactions.write.out | Number of concurrent write transaction in progress. | long | gauge |
| mongodb.status.wired_tiger.concurrent_transactions.write.total_tickets | Number of total write tickets. | long | gauge |
| mongodb.status.wired_tiger.log.flushes | Number of flush operations. | long | counter |
| mongodb.status.wired_tiger.log.max_file_size.bytes | Maximum file size. | long | gauge |
| mongodb.status.wired_tiger.log.scans | Number of scan operations. | long | counter |
| mongodb.status.wired_tiger.log.size.bytes | Total log size in bytes. | long | gauge |
| mongodb.status.wired_tiger.log.syncs | Number of sync operations. | long | counter |
| mongodb.status.wired_tiger.log.write.bytes | Number of bytes written into the log. | long | counter |
| mongodb.status.wired_tiger.log.writes | Number of write operations. | long | counter |
| mongodb.status.write_backs_queued | True when there are operations from a mongos instance queued for retrying. | boolean |  |
| process.name | Process name. Sometimes called program name or similar. | keyword |  |
| process.name.text | Multi-field of `process.name`. | match_only_text |  |
| service.address | Address of the machine where the service is running. | keyword |  |
| service.type | The type of the service data is collected from. The type can be used to group and correlate logs and metrics from one service type. Example: If logs or metrics are collected from Elasticsearch, `service.type` would be `elasticsearch`. | keyword |  |
| service.version | Version of the service the data was collected from. This allows to look at a data set only for a specific version of a service. | keyword |  |

