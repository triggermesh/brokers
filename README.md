# TriggerMesh Brokers

TriggerMesh supported brokers.

## Configuration

Configuration (WIP) informs about the Triggers that send events to targets.
Durations follow [ISO 8601](https://en.wikipedia.org/wiki/ISO_8601) format

```yaml
triggers:
  trigger1:
    filters:
    - exact:
        type: example.type
    target:
      url: http://localhost:8888
      deliveryOptions:
        retry: 2
        backoffDelay: PT2S
        backoffPolicy: linear
  trigger2:
    target:
      url: http://localhost:9999
      deliveryOptions:
        retry: 5
        backoffDelay: PT5S
        backoffPolicy: constant
        deadLetterURL: http://localhost:9090
```

## Usage

Produce CloudEvents by sending then using an HTTP client.

```console
curl -v  http://localhost:8080/ \
  -H "Ce-Specversion: 1.0" \
  -H "Ce-Type: example.type" \
  -H "Ce-Source: example.source" \
  -H "Ce-Id: 1234-abcd-x" \
  -H "Content-Type: application/json" \
  -d '{"hello":"broker"}'
```

## Redis

Redis Broker needs a Redis backing server to perform pub/sub operations and storage.

The broker uses a single Redis stream named `triggermesh` by default, that can be customized using `redis.stream` argument.
The Redis user must be configured to use the `stream` group of commands on the stream key, plus using the `client` command with `id` subcomand for probes.

When using a single Redis backend, it is important to use a unique stream per broker to isolate messages.

```console
# In this example the broker will be configured with user triggermesh1
# and stream name triggermeshstream

ACL SETUSER triggermesh1 on >7r!663R +@stream +client|id ~triggermeshstream
```

### Non Authenticated Redis

```console
# Create storage folder
mkdir -p .local/data

# Run Redis alternative
docker run -d -v $PWD/.local/data:/data \
    -e REDIS_ARGS="--appendonly yes --appendfsync always --rdbcompression yes" \
    --name redis-stack-server \
    -p 6379:6379 \
    redis/redis-stack-server:latest
```

Launch the broker providing parameters for the backing server.

```console
go run ./cmd/redis-broker start \
  --redis.address "0.0.0.0:6379" \
  --broker-config-path ".local/broker-config.yaml"
```

### Authenticated Redis

When using an authenticated Redis instance, user and password can be informed via `redis.username` and `redis.password` arguments.

```console
go run ./cmd/redis-broker start \
  --redis.username triggermesh1 \
  --redis.password "7r\!663R" \
  --redis.address "some.redis.server:25101" \
  --broker-config-path .local/broker-config.yaml
```

### TLS Enabled Redis

If the Redis instance is exposed using TLS, it must enabled at the broker config via `redis.tls-enabled` flag. When using self-signed certificates `redis.tls-skip-verify` must be used.

```console
go run ./cmd/redis-broker start \
  --redis.username triggermesh1 \
  --redis.password "7r\!663R" \
  --redis.tls-enabled  \
  --redis.tls-skip-verify \
  --redis.address "tls.self.signed.redis.server:25102" \
  --broker-config-path .local/broker-config.yaml
```

### Using Environment Variables

Parameters for the broker can be set as environment variables.

```console
BROKER_CONFIG_PATH=.local/broker-config.yaml \
REDIS_ADDRESS=tls.self.signed.redis.server:25102 \
REDIS_USERNAME=triggermesh1 \
REDIS_PASSWORD=7r\!663R \
REDIS_TLS_ENABLED=true \
REDIS_TLS_SKIP_VERIFY=true \
go run ./cmd/redis-broker start
```

## Memory

```console
go run ./cmd/memory-broker start --memory.buffer-size 100 --memory.produce-timeout 1s --broker-config-path ".local/config.yaml"
```

Alternatively environment variables could be used.

```console
CONFIG_PATH=.local/config.yaml MEMORY_BUFFER_SIZE=100 MEMORY_PRODUCE_TIMEOUT=1s go run ./cmd/memory-broker start
```

## Container

```console
docker build -t my-repo/redis-broker:my-version .
docker push my-repo/redis-broker:my-version
```

## Observability

The `observability-config-path` flag allows you to customize observability settings.

```console
go run ./cmd/redis-broker start --redis.address "0.0.0.0:6379" \
  --broker-config-path .local/broker-config.yaml \
  --observability-config-path .local/observability-config.yaml
```

The file contains a `zap-logger-config` element where a zap configuration should be located. Updating the file will update the logging level.

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
```

## Parameters

### Arguments

Prefixes `redis.` and `memory.` apply only to their respective broker binaries.

Name | Default | Information
--- | --- | ---
redis.address |  |
broker-config-path | /etc/triggermesh/broker.conf | Path to the file that contains the broker's configuration.
### Environment Variables

Name                                  | Default | Information
---                                   | --- | ---
REDIS_ADDRESS                         |  | Redis backend address, including port.
KUBERNETES_NAMESPACE                  |  | Kubernetes namespace where the broker runs.
BROKER_CONFIG_KUBERNETES_SECRET_NAME  | | Kubernetes Secret name for the broker configuration.
BROKER_CONFIG_KUBERNETES_SECRET_KEY   | | Kubernetes Secret key for the broker configuration.
OBSERVABILITY_CONFIG_KUBERNETES_CONFIGMAP_NAME | | Kubernetes ConfigMap name for the observability configuration.


## Generate License

Install `addlicense`:

```console
go install github.com/google/addlicense@v1.0.0
```

Make sure all files contain a license

```console
addlicense -c "TriggerMesh Inc." -y $(date +"%Y") -l apache -s=only ./**/*.go
```
