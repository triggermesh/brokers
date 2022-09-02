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

	// Create backend client.
	b := redis.New(&c.Redis, globals.logger.Named("redis"))

	// Create the subscription manager.
	sm := subscriptions.New(globals.logger.Named("subs"))

	// Create ingest server. Register the CloudEvents
	// handler to send received CE to the backend.
	i := ingest.NewInstance(globals.logger.Named("ingest"))
	i.RegisterCloudEventHandler(b.Produce)

	// TODO register probes
	// TODO register subscriptor

	// The ConfigWatcher will read the configfile and call registered
	// callbacks upon start and everytime the configuration file
	// is updated.
	cfw, err := fs.NewCachedFileWatcher(globals.logger.Named("fswatch"))
	if err != nil {
		return err
	}
	cfgw := config.NewWatcher(cfw, c.ConfigPath, globals.logger.Named("cgfwatch"))

	// ConfigWatcher will callback reconfigurations for:
	// - Ingest: if authentication parameters are updated.
	// - Subscription manager: if triggers configurations changes.
	cfgw.AddCallback(i.UpdateFromConfig)
	cfgw.AddCallback(sm.UpdateFromConfig)

	// Create broker to start all runtimer elements
	// an manage signaling
	bi := broker.NewInstance(b, i, sm, cfgw, globals.logger)

	return bi.Start(globals.context)
}
