// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package redis

import "time"

type RedisArgs struct {
	Address       string `help:"Redis address." env:"ADDRESS" default:"0.0.0.0:6379"`
	Username      string `help:"Redis username." env:"USERNAME"`
	Password      string `help:"Redis password." env:"PASSWORD"`
	Database      int    `help:"Database ordinal at Redis." env:"DATABASE" default:"0"`
	TLSEnabled    bool   `help:"TLS enablement for Redis connection." env:"TLS_ENABLED" default:"false"`
	TLSSkipVerify bool   `help:"TLS skipping certificate verification." env:"TLS_SKIP_VERIFY" default:"false"`

	Stream            string        `help:"Stream name that stores the broker's CloudEvents." env:"STREAM" default:"triggermesh"`
	Group             string        `help:"Redis stream consumer group name." env:"GROUP" default:"default"`
	Instance          string        `help:"Instance name at the Redis stream consumer group." env:"INSTANCE" default:"${instance_name}"`
	StreamMaxLen      int           `help:"Limit the number of items in a stream by trimming it." env:"STREAM_MAXLEN" default:"0"`
	ProcessingTimeout time.Duration `help:"Time after which an event that did not complete processing will be re-delivered by Redis." env:"PROCESSING_TIMEOUT" default:"3m"`
}
