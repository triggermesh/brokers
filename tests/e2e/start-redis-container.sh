#!/bin/bash

mkdir -p .local/data

docker run -d -v ".local/data:/data" \
    -e REDIS_ARGS="--appendonly yes" \
    --name redis-stack-server \
    -p 6379:6379 \
    redis/redis-stack-server:latest