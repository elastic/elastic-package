# ECS Mappings Test

Test package to verify support for ECS mappings available to integrations running on stack version 8.13.0 and later.

Please note that the package:

- does not embed the legacy ECS mappings (no `import_mappings`).
- does not define fields in the `ecs.yml` file. 

Mappings for ECS fields (for example, `ecs.version`) come from the `ecs@mappings` component template in the integration index template, which has been available since 8.13.0. 

An example event for `first` looks as following:

```json
{
    "source.geo.location": {
        "lat": 1.0,
        "lon": "2.0"
    },
    "destination.geo.location.lat": 3.0,
    "destination.geo.location.lon": 4.0,
    "service.status.duration.histogram": {
        "counts": [
            8,
            17,
            8,
            7,
            6,
            2
        ],
        "values": [
            0.1,
            0.25,
            0.35,
            0.4,
            0.45,
            0.5
        ]
    },
    "ecs": {
        "version": "8.11.0"
    }
}
```

**Exported fields**

| Field | Description | Type |
|---|---|---|
| @timestamp | Event timestamp. | date |
| data_stream.dataset | Data stream dataset. | constant_keyword |
| data_stream.namespace | Data stream namespace. | constant_keyword |
| data_stream.type | Data stream type. | constant_keyword |
| service.status.\*.histogram |  | object |

