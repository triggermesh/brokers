// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/triggermesh/brokers/pkg/backend/impl/memory"
	"github.com/triggermesh/brokers/pkg/broker"
	pkgcmd "github.com/triggermesh/brokers/pkg/cmd"
	"github.com/triggermesh/brokers/pkg/common/fs"
	cfgwatcher "github.com/triggermesh/brokers/pkg/config/watcher"
	"github.com/triggermesh/brokers/pkg/ingest"
	"github.com/triggermesh/brokers/pkg/subscriptions"
)

type StartCmd struct {
	ConfigPath string `help:"Path to configuration file." env:"CONFIG_PATH" default:"/etc/triggermesh/broker.conf"`

	Memory memory.MemoryArgs `embed:"" prefix:"memory." envprefix:"MEMORY_"`
}

func (c *StartCmd) Run(globals *pkgcmd.Globals) error {
	globals.Logger.Info("Starting gateway")

	// Create backend client.
	b := memory.New(&c.Memory, globals.Logger.Named("memory"))

	// Create the subscription manager.
	sm, err := subscriptions.New(globals.Logger.Named("subs"), b)
	if err != nil {
		return err
	}

	// Create ingest server.
	i := ingest.NewInstance(globals.Logger.Named("ingest"))

	// The ConfigWatcher will read the configfile and call registered
	// callbacks upon start and everytime the configuration file
	// is updated.
	cfw, err := fs.NewCachedFileWatcher(globals.Logger.Named("fswatch"))
	if err != nil {
		return err
	}

	configPath, err := filepath.Abs(c.ConfigPath)
	if err != nil {
		return fmt.Errorf("error resolving to absoluthe path %q: %w", c.ConfigPath, err)
	}

	cfgw, err := cfgwatcher.NewWatcher(cfw, configPath, globals.Logger.Named("cgfwatch"))
	if err != nil {
		return err
	}

	// Create broker to start all runtimer elements
	// an manage signaling
	bi := broker.NewInstance(b, i, sm, cfgw, globals.Logger)

	return bi.Start(globals.Context)
}
