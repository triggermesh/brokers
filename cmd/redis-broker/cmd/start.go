// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/triggermesh/brokers/pkg/backend/impl/redis"
	"github.com/triggermesh/brokers/pkg/broker"
	"github.com/triggermesh/brokers/pkg/common/fs"
	"github.com/triggermesh/brokers/pkg/config"
	"github.com/triggermesh/brokers/pkg/ingest"
	"github.com/triggermesh/brokers/pkg/subscriptions"
)

type StartCmd struct {
	InstanceName string `help:"Gateway instance name." default:"default"`
	ConfigPath   string `help:"Path to configuration file." default:"/etc/triggermesh/gateway.conf"`

	Redis redis.RedisArgs `embed:"" prefix:"redis."`
}

func (c *StartCmd) Run(globals *Globals) error {
	globals.logger.Info("Starting gateway")

	// Create backend client
	b := redis.New(globals.logger, &c.Redis)

	// Create ingest server. A reference to the backend is passed
	// to produce new events from the server.
	i := ingest.NewInstance(b, globals.logger)

	// Create the subscription manager. A reference to the backend
	// is passed to receive and dispatch events.
	sm := subscriptions.New(b, globals.logger)

	// The ConfigWatcher will read the configfile and call registered
	// callbacks upon start and everytime the configuration file
	// is updated.
	cfw, err := fs.NewCachedFileWatcher(globals.logger)
	if err != nil {
		return err
	}
	cfgw := config.NewWatcher(cfw, c.ConfigPath, globals.logger)

	// ConfigWatcher will callback reconfigurations for:
	// - Ingest: if authentication parameters are updated.
	// - Subscription manager: if triggers configurations changes.
	cfgw.AddCallback(i.UpdateFromConfig)
	cfgw.AddCallback(sm.UpdateFromConfig)

	// TODO Create broker to coordinate all of the above
	// an manage signaling
	bi := broker.NewInstance(b, i, sm, globals.logger)

	return bi.Start(globals.context)
}
