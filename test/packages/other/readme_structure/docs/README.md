# Test Integration

### Compatibility

Compatible with Linux, Windows, macOS, and imaginary systems.

## What data does this integration collect?

This integration collects basic event logs, dummy metrics, and simulated user actions.

### Supported use cases

- Testing pipeline connectivity  
- Simulating event ingestion  
- Dummy dashboard population

## What do I need to use this integration?

- A running instance of the test agent  
- Dummy API credentials (username: `test`, password: `dummy`)  
- Optional: a container or VM to simulate load

## How do I deploy this integration?

1. Install the test integration package.  
2. Configure the dummy data source.  
3. Enable the integration via the dashboard.  
4. Verify events are received.

## Troubleshooting

- If no data is shown, ensure the dummy service is running.  
- Check logs at `/var/log/dummy.log` for errors.  
- Restart the test agent if needed.

## Performance and scaling

This integration is lightweight and scales linearly with event volume.  
For high-load testing, deploy across multiple nodes.

## Reference

### ECS field reference

**Exported fields**

| Field       | Description         | Type    |
|-------------|---------------------|---------|
| `@timestamp`| Event timestamp.    | `date`  |
| `message`   | Dummy log message.  | `text`  |
| `event.type`| Type of event.      | `keyword` |

### Example event

```json
{
  "@timestamp": "2012-10-30T09:46:12.000Z",
  "message": "Dummy event triggered",
  "event.type": "info"
}
