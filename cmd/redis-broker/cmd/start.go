// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/triggermesh/brokers/pkg/backend/impl/redis"
	"github.com/triggermesh/brokers/pkg/broker"
	pkgcmd "github.com/triggermesh/brokers/pkg/broker/cmd"
)

type StartCmd struct {
	Redis redis.RedisArgs `embed:"" prefix:"redis." envprefix:"REDIS_"`
}

func (c *StartCmd) Run(globals *pkgcmd.Globals) error {
	globals.Logger.Debug("Creating Redis backend client")

	// Use InstanceName as Redis instance at the consumer group.
	c.Redis.Instance = globals.InstanceName
	backend := redis.New(&c.Redis, globals.Logger.Named("redis"))

	b, err := broker.NewInstance(globals, backend)
	if err != nil {
		return err
	}

	return b.Start(globals.Context)
}
