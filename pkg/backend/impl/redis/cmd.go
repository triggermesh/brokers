// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"fmt"
	"strings"
)

type RedisArgs struct {
	Address          string   `help:"Redis address." env:"ADDRESS" default:"0.0.0.0:6379"`
	ClusterAddresses []string `help:"Redis address." env:"CLUSTER_ADDRESSES"`

	Username      string `help:"Redis username." env:"USERNAME"`
	Password      string `help:"Redis password." env:"PASSWORD"`
	Database      int    `help:"Database ordinal at Redis." env:"DATABASE" default:"0"`
	TLSEnabled    bool   `help:"TLS enablement for Redis connection." env:"TLS_ENABLED" default:"false"`
	TLSSkipVerify bool   `help:"TLS skipping certificate verification." env:"TLS_SKIP_VERIFY" default:"false"`

	Stream string `help:"Stream name that stores the broker's CloudEvents." env:"STREAM" default:"triggermesh"`
	Group  string `help:"Redis stream consumer group name." env:"GROUP" default:"default"`
	// Instance at the Redis stream consumer group. Copied from the InstanceName at the global args.
	Instance string `kong:"-"`

	StreamMaxLen int `help:"Limit the number of items in a stream by trimming it. Set to 0 for unlimited." env:"STREAM_MAX_LEN" default:"1000"`
}

func (ra *RedisArgs) Validate() error {
	msg := []string{}

	// Since there is a default value at addresses, we only check that cluster addresses and a value for
	// and standalone instance that is different to the default must not be provided.
	if len(ra.ClusterAddresses) != 0 &&
		ra.Address != "0.0.0.0:6379" && ra.Address != "" {
		msg = append(msg, "Only one of address (standalone) or cluster addresses (cluster) arguments must be provided.")
	}

	if len(msg) == 0 {
		return nil
	}

	return fmt.Errorf(strings.Join(msg, " "))
}
