// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package redis

import "time"

type RedisArgs struct {
	Address  string `help:"Redis address." default:"0.0.0.0:6379"`
	Password string `help:"Redis password."`
	Database int    `help:"Database ordinal at Redis." default:"0"`

	Stream            string        `help:"Stream name that stores the gateway CloudEvents." default:"default"`
	Group             string        `help:"Redis stream consumer group name." default:"default"`
	Instance          string        `help:"Instance name at the Redis stream consumer group." default:"${instance_name}"`
	ProcessingTimeout time.Duration `help:"Time after which an event that did not complete processing will be re-delivered by Redis." default:"3m"`
}
