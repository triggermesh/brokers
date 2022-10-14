# E2E tests

```console
go test --count 1 -v ./test/e2e/... -tags e2e
```

## Redis tests

It is expected that a Redis server will be running locally when e2e tests are run.
Redis tests can be customized using these parameters:

```text
--redis-address my.redis.server:6379    # For using an explicit Redis server
--redis-password pA55vv04d              # To use a Redis password
--redis-stream mystream                 # To use an explicit Redis stream
```
