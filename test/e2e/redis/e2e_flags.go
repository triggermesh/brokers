package redis

import (
	"flag"
)

type RedisTestsFlags struct {
	RedisAddress  string
	RedisPassword string
	RedisStream   string
}

var Flags RedisTestsFlags

func InitializeRedisFlags() {
	flag.StringVar(&Flags.RedisAddress, "redis-address", "0.0.0.0:6379", "Redis server address")
	flag.StringVar(&Flags.RedisPassword, "redis-password", "", "Redis server password")
	flag.StringVar(&Flags.RedisStream, "redis-stream", "", "Redis server stream")

	flag.Parse()
}
