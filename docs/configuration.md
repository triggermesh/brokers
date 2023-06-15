# (Draft) Configuration

## Broker Configuration

```yaml
triggers: <TRIGGER LIST>
  <TRIGGER-NAME>:
    filters:
    <FILTERS ARRAY>
    bounds:
      startId: <BACKEND ID FOR THE FIRST ELEMENT TO RECEIVE>
      endId: <BACKEND ID FOR THE LAST ELEMENT TO RECEIVE>
    target:
      url: <DESTINATION URL>
    deliveryOptions:
      retry: <RETRIES WHEN FAILED TO DELIVER>
      backoffDelay: <RETRY DELAY FACTOR AS ISO 8601 DURATION>
      backoffPolicy: <RETRY BACKOFF POLICY>
      deadLetterURL: <DEAD LETTER URL>
```

The configuration's root `triggers` element contains a set of triggers listed under their names:

```yaml
triggers:
  some-trigger:
    ...
  some-other-trigger:
    ...
```

Triggers configure subscriptions from the broker to a target, using optional filter and message bounds.

- Target: is the URL where events will be sent.
- Filter: use CloudEvents attribute filters
- Bounds: use an `startId` and `endId` bound value.

A bounded trigger can be created to replay events. Only Redis broker is capable of replaying events, and bounds are set after the internal Unix timestamp with millisecond precision (example `1686851697104-0`). Bounds for the redis broker are exclusive, start and end IDs are not sent to the target.

## Broker Configuration Examples

### Example 1

- Only allow CloudEvents type `example.type`
- Send to `http://localhost:9000`
- Retry 2 times, backing off liearly with 5 seconds factor.

```yaml
triggers:
  trigger1:
    filters:
    - exact:
        type: example.type
    target:
      url: http://localhost:9000
    deliveryOptions:
      retry: 2
      backoffDelay: PT5S
      backoffPolicy: linear
```

## Example Replay

```yaml
triggers:
  replay1:
    bounds:
      startId: 1686851639344-0
      endId: 1686851697104-0
    filters:
    - exact:
        type: example.type
    target:
      url: http://localhost:9099
    deliveryOptions:
      retry: 1
      backoffDelay: PT5S
      backoffPolicy: linear
```

## Observability Examples

### Example 1

- Info logging level.
- Prometheus metrics exported at 9092

```yaml
zap-logger-config: |
  {
    "level": "info",
    "development": false,
    "outputPaths": ["stdout"],
    "errorOutputPaths": ["stderr"],
    "encoding": "json",
    "encoderConfig": {
      "timeKey": "timestamp",
      "levelKey": "severity",
      "nameKey": "logger",
      "callerKey": "caller",
      "messageKey": "message",
      "stacktraceKey": "stacktrace",
      "lineEnding": "",
      "levelEncoder": "",
      "timeEncoder": "iso8601",
      "durationEncoder": "",
      "callerEncoder": ""
    }
  }

# NOTE: Only one of the next 2 blocks, prometheus or opencensus, should be informed.

# Enables the Prometheus metrics exporter.
# Exposes telemetry metrics in a text-based format on the HTTP endpoint :9092/metrics.
metrics.backend-destination: prometheus
metrics.prometheus-port: 9092
metrics.reporting-period-seconds: 5
```

### Example 2

- Info logging debug.
- Opencensus metrics sent to `otel-collector.metrics:55678`

```yaml
zap-logger-config: |
  {
    "level": "debug",
    "development": false,
    "outputPaths": ["stdout"],
    "errorOutputPaths": ["stderr"],
    "encoding": "json",
    "encoderConfig": {
      "timeKey": "timestamp",
      "levelKey": "severity",
      "nameKey": "logger",
      "callerKey": "caller",
      "messageKey": "message",
      "stacktraceKey": "stacktrace",
      "lineEnding": "",
      "levelEncoder": "",
      "timeEncoder": "iso8601",
      "durationEncoder": "",
      "callerEncoder": ""
    }
  }

# Enables the opencensus metrics integration in all TriggerMesh components.
metrics.backend-destination: opencensus
metrics.opencensus-address: otel-collector.metrics:55678
metrics.reporting-period-seconds: 30
```

## Inline Broker Configuration

For environments where there is no disk for running the broker, a static configuration can be specified using either the command line argument `broker-config` or the environment variable `BROKER_CONFIG`.

```console
go run ./cmd/redis-broker start \
  --redis.address "0.0.0.0:6379" \
  --broker-config '
  {"triggers": {
    "trigger1": {
      "filters": [
        {
          "exact": {
            "type": "example.type"
          }
        }
      ],
      "target": {
        "url": "http://localhost:9000",
        "deliveryOptions": {
          "retry": 2,
          "backoffDelay": "PT5S",
          "backoffPolicy": "linear"
        }
      }
    }
  }
}'
```