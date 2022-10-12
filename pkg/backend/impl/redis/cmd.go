// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package redis

import "time"

type RedisArgs struct {
	Address  string `help:"Redis address." env:"ADDRESS" default:"0.0.0.0:6379"`
	Password string `help:"Redis password." env:"PASSWORD"`
	Database int    `help:"Database ordinal at Redis." env:"DATABASE" default:"0"`

	Stream            string        `help:"Stream name that stores the broker's CloudEvents." env:"STREAM" default:"default"`
	Group             string        `help:"Redis stream consumer group name." env:"GROUP" default:"default"`
	Instance          string        `help:"Instance name at the Redis stream consumer group." env:"INSTANCE" default:"${instance_name}"`
	ProcessingTimeout time.Duration `help:"Time after which an event that did not complete processing will be re-delivered by Redis." env:"BACKEND_PROCESSING_TIMEOUT" default:"3m"`
}
