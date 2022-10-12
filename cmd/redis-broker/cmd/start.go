// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/triggermesh/brokers/pkg/backend/impl/redis"
	pkgcmd "github.com/triggermesh/brokers/pkg/cmd"
)

type StartCmd struct {
	Redis redis.RedisArgs `embed:"" prefix:"redis." envprefix:"REDIS_"`
}

func (c *StartCmd) Run(globals *pkgcmd.Globals) error {
	globals.Logger.Debug("Creating Redis backend client")
	b := redis.New(&c.Redis, globals.Logger.Named("redis"))

	return pkgcmd.Run(globals, b)
}
