// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"fmt"
	"strings"
)

type RedisArgs struct {
	Address       string `help:"Redis address." env:"ADDRESS" default:"0.0.0.0:6379"`
	Username      string `help:"Redis username." env:"USERNAME"`
	Password      string `help:"Redis password." env:"PASSWORD"`
	Database      int    `help:"Database ordinal at Redis." env:"DATABASE" default:"0"`
	TLSEnabled    bool   `help:"TLS enablement for Redis connection." env:"TLS_ENABLED" default:"false"`
	TLSSkipVerify bool   `help:"TLS skipping certificate verification." env:"TLS_SKIP_VERIFY" default:"false"`

	Stream string `help:"Stream name that stores the broker's CloudEvents." env:"STREAM" default:"triggermesh"`
	Group  string `help:"Redis stream consumer group name." env:"GROUP" default:"default"`
	// Instance at the Redis stream consumer group. Copied from the InstanceName at the global args.
	Instance string `kong:"-"`

	StreamMaxLen int `help:"Limit the number of items in a stream by trimming it. Set to 0 for unlimited." env:"STREAM_MAXLEN" default:"0"`
}

func (ra *RedisArgs) Validate() error {
	msg := []string{}

	// TODO add validations

	if len(msg) == 0 {
		return nil
	}

	return fmt.Errorf(strings.Join(msg, " "))
}
