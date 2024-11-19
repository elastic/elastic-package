# Long Integer Tests

An example event for `test` looks as following:

```json
{
    "@warning": "The values in sequence_number must match the values in message.",
    "message": "4503599627370497,9007199254740991,9007199254740993,18014398509481985,9223372036854773807",
    "sequence_number": [
        4503599627370497,
        9007199254740991,
        9007199254740993,
        18014398509481985,
        9223372036854773807
    ]
}
```

**Exported fields**

| Field | Description | Type |
|---|---|---|
| @timestamp | Event timestamp. | date |
| @warning | Warning for devs. | keyword |
| data_stream.dataset | Data stream dataset. | constant_keyword |
| data_stream.namespace | Data stream namespace. | constant_keyword |
| data_stream.type | Data stream type. | constant_keyword |
| message | Original input. | keyword |
| sequence_number | Log entry identifier that is incremented sequentially. Unique for each log type. | long |
