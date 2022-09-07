# TriggerMesh Brokers

TriggerMesh supported brokers.

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

Configuration (WIP) informs about the Triggers that send events to targets.

```yaml
ingest:
  user: user11
  password: password1

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
```

Launch the broker providing parameters for the backing server.

```console
go run ./cmd/redis-broker start --redis.address "0.0.0.0:6379" --config-path ".local/config.yaml"
```

## Memory

## Generate License

Install `addlicense`:

```console
go install github.com/google/addlicense@v1.0.0
```

Make sure all files contain a license

```console
addlicense -c "TriggerMesh Inc." -y $(date +"%Y") -l apache -s=only ./**/*.go
```
