# Configuration

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
