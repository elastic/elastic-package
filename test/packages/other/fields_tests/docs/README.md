# Fields Tests

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
| destination.geo.location | Longitude and latitude. | geo_point |
| service.status.\*.histogram |  | object |
| source.geo.location | Longitude and latitude. | geo_point |
