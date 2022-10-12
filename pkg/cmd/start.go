// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/triggermesh/brokers/pkg/backend"
	"github.com/triggermesh/brokers/pkg/broker"
	"github.com/triggermesh/brokers/pkg/common/fs"
	cfgwatcher "github.com/triggermesh/brokers/pkg/config/watcher"
	"github.com/triggermesh/brokers/pkg/ingest"
	"github.com/triggermesh/brokers/pkg/subscriptions"
	"go.uber.org/zap"
)

func Run(globals *Globals, b backend.Interface) error {
	globals.Logger.Info("Starting broker")

	globals.Logger.Debug("Creating subscription manager")
	sm, err := subscriptions.New(globals.Logger.Named("subs"), b)
	if err != nil {
		return err
	}

	globals.Logger.Debug("Creating HTTP ingest server")
	i := ingest.NewInstance(globals.Logger.Named("ingest"))

	// The ConfigWatcher will read the configfile and call registered
	// callbacks upon start and everytime the configuration file
	// is updated.
	cfw, err := fs.NewCachedFileWatcher(globals.Logger.Named("fswatch"))
	if err != nil {
		return err
	}

	configPath, err := filepath.Abs(globals.ConfigPath)
	if err != nil {
		return fmt.Errorf("error resolving to absolute path %q: %w", globals.ConfigPath, err)
	}

	globals.Logger.Debugw("Creating watcher for broker configuration", zap.String("file", configPath))
	cfgw, err := cfgwatcher.NewWatcher(cfw, configPath, globals.Logger.Named("cgfwatch"))
	if err != nil {
		return err
	}

	globals.Logger.Debug("Creating broker instance")
	bi := broker.NewInstance(b, i, sm, cfgw, globals.Logger.Named("broker"))

	return bi.Start(globals.Context)
}