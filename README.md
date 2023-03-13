[![Release](https://img.shields.io/github/v/release/triggermesh/brokers?label=release)](https://github.com/triggermesh/brokers/releases)
[![Slack](https://img.shields.io/badge/Slack-Join%20chat-4a154b?style=flat&logo=slack)](https://join.slack.com/t/triggermesh-community/shared_invite/zt-1kngevosm-MY7kqn9h6bT08hWh8PeltA)

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
        deadLetterURL: http://localhost:9000
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

### Redis Replay

Redis Broker comes with an additional tool `redis-replay.` This tool allows users to replay/re-inject historical events. 

The `redis-replay` can be invoked locally via exporting the required variables into your enviroment and then then runing it from the `cmd/redis-broker` folder. 

The required enviorment variables are:

* REDIS_ADDRESS - The location of the Redis backend (It should be shared with the Broker.) 
* REDIS_PASSWORD - (If configured) The Redis Password
* REDIS_USER - (If configured) The Redis User
* REDIS_DATABASE - The Redis Database
* K_SINK - Where Events should be replayed to.
* START_TIME - (Optional) The time to begin replaying events from (ex. "2023-02-13T16:01:12Z")
* END_TIME - (Optional) The time that events should be replayed up-untill (ex "2023-05-13T16:01:12Z")
* FILTER_KIND - (Optional) The kind of event filtering to perform. Can be one of: 
    * type 
    * source
    * subject
    * datacontenttype
    * id
* FILTER - (Optional) The value to check against after selecting the FILTER_KIND. (I.E. If one selected `type` for the FILTER_KIND `io.example.message` might be used for FILTER)


Once Redis Broker is running, and has some events. You can now run the redis-replay locally:
```
export REDIS_ADDRESS=localhost:6666
export REDIS_PASSWORD=""
export REDIS_DATABASE="0"
export K_SINK=http://localhost:8082
export START_TIME="2023-02-13T16:01:12Z"
export END_TIME="2023-05-13T16:01:12Z"
export FILTER=id
export FILTER_KIND=1234-abcd-x
export REDIS_PASSWORD=test
export REDIS_USER=default
go run cmd/replay/redis/main.go 
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

Note: when using a Redis cluster provide a comma separated list of nodes at `REDIS_CLUSTER_ADDRESSES` instead of the `REDIS_ADDRESS` parameter.

## Memory

```console
go run ./cmd/memory-broker start --memory.buffer-size 100 --memory.produce-timeout 1s --broker-config-path ".local/config.yaml"
```

Alternatively environment variables could be used.

```console
CONFIG_PATH=.local/config.yaml MEMORY_BUFFER_SIZE=100 MEMORY_PRODUCE_TIMEOUT=1s go run ./cmd/memory-broker start
```

## Container Images

```console
docker build -t my-repo/redis-broker:my-version .
docker push my-repo/redis-broker:my-version

docker build -t my-repo/memory-broker:my-version .
docker push my-repo/memory-broker:my-version
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

## Broker Parameters

Prefixes `redis.` and `memory.` apply only to their respective broker binaries.

Name | Environment | Default | Information
--- | --- | --- | ---
broker-config-path        | BROKER_CONFIG_PATH              | /etc/triggermesh/broker.conf | Path to broker configuration file.
observability-config-path | OBSERVABILITY_CONFIG_PATH       | | Path to observability configuration file.
port                      | PORT                            | 8080 | HTTP Port to listen for CloudEvents.
broker-name             | BROKER_NAME                   |`{hostname}` | Instance name. When running at Kubernetes should be set to RedisBroker name.
kubernetes-namespace      | KUBERNETES_NAMESPACE            | | Namespace where the broker is running.
kubernetes-broker-config-secret-name  | KUBERNETES_BROKER_CONFIG_SECRET_NAME | | Secret object name that contains the broker configuration.
kubernetes-broker-config-secret-key   | KUBERNETES_BROKER_CONFIG_SECRET_KEY  | | Secret object key that contains the broker configuration.
kubernetes-observability-config-map-name  | KUBERNETES_OBSERVABILITY_CONFIGMAP_NAME || ConfigMap object name that contains the observability configuration.
config-polling-period                 | CONFIG_POLLING_PERIOD    | PT0S | ISO8601 duration for config polling. Disabled if PT0S. Enabling it will disable other configuration methods.
broker-config                 | BROKER_CONFIG    | | JSON representation of broker configuration. Enabling it will disable other configuration methods.
observability-config                 | BROKER_CONFIG    |  | JSON representation of observability configuration. Enabling it will disable other configuration methods.
observability-metrics-domain          | OBSERVABILITY_CONFIG  | triggermesh.io/eventing | Domain to be used for some metrics reporters.
redis.address             | REDIS_ADDRESS                   | 0.0.0.0:6379 | Redis address for standalone instances.
redis.cluster-addresses   | REDIS_CLUSTER_ADDRESSES         | | Comma separated list of redis addresses for clustered instances.
redis.username            | REDIS_USERNAME                  | | Redis username.
redis.password            | REDIS_PASSWORD                  | | Redis password.
redis.database            | REDIS_DATABASE                  | 0 | Database ordinal at Redis.
redis.tls-enabled         | REDIS_TLS_ENABLED               | false | TLS enablement for Redis connection.
redis.tls-skip-verify     | REDIS_TLS_SKIP_VERIFY           | false | TLS skipping certificate verification.
redis.stream              | REDIS_STREAM                    | triggermesh | Stream name that stores the broker's CloudEvents.
redis.group               | REDIS_GROUP                     | default | Redis stream consumer group name.
redis.stream-max-len      | REDIS_STREAM_MAX_LEN            | 1000 | Limit the number of items in a stream by trimming it. Set to 0 for unlimited.
memory.buffer-size        | MEMORY_BUFFER_SIZE              | 10000 | Number of events that can be hosted in the backend.
memory.produce-timeout    | MEMORY_PRODUCE_TIMEOUT          | PT5S | Maximum wait time for producing an event to the backend. Formatted as ISO8601 duration.

## Generate License

Install `addlicense`:

```console
go install github.com/google/addlicense@v1.0.0
```

Make sure all files contain a license

```console
addlicense -c "TriggerMesh Inc." -y $(date +"%Y") -l apache -s=only ./**/*.go
```
