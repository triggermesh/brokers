#!/bin/bash

mkdir -p .local/data

docker run -d -v $PWD/.local/data:/data \
    -e REDIS_ARGS="--appendonly yes" \
    --name redis-stack-server \
    -p 6379:6379 \
    redis/redis-stack-server:latest

for i in {1..50}
do
  echo "$i try to connect to Redis."
  res=`(printf "PING\r\n"); sleep 1 | nc localhost 6379 -w1`
  if [[ $res == +PONG* ]]; then
    echo "Redis OK."
    exit 0
  fi
done

exit 6
