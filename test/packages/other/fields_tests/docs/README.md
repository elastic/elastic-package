# Fields Tests

An example event for `first` looks as following:

```json
{
    "source.geo.location": {
        "lat": 1.0,
        "lon": "2.0"
    },
    "geo.location.lat": 3.0,
    "geo.location.lon": 4.0
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
| geo.location | Longitude and latitude. | geo_point |
| source.geo.location | Longitude and latitude. | geo_point |
