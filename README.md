# TriggerMesh Brokers

TriggerMesh supported brokers.

## Configuration

Configuration (WIP) informs about the Triggers that send events to targets.

```yaml
triggers:
- name: trigger1
  filters:
  - exact:
      type: example.type
  targets:
  - url: http://localhost:8888
    deliveryOptions:
      retries: 2
      backoffDelay: 2s
      backoffPolicy: linear
- name: trigger2
  targets:
  - url: http://localhost:9999
    deliveryOptions:
      retries: 5
      backoffDelay: 5s
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

```console
# Create storage folder
mkdir -p .local/data

# Run Redis
docker run -d -v $PWD/.local/data:/data \
    -e REDIS_ARGS="--appendonly yes" \
    --name redis-stack-server \
    -p 6379:6379 \
    redis/redis-stack-server:latest
```

Launch the broker providing parameters for the backing server.

```console
go run ./cmd/redis-broker start --redis.address "0.0.0.0:6379" --config-path ".local/config.yaml"
```

## Memory

```console
go run ./cmd/memory-broker start --memory.buffer-size 100 --memory.produce-timeout 1s --config-path ".local/config.yaml"
```

## Generate License

Install `addlicense`:

```console
go install github.com/google/addlicense@v1.0.0
```

Make sure all files contain a license

```console
addlicense -c "TriggerMesh Inc." -y $(date +"%Y") -l apache -s=only ./**/*.go
```
