// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/triggermesh/brokers/pkg/common/fs"
	"github.com/triggermesh/brokers/pkg/config"
)

type WatcherCallback func(*config.Config)

type Watcher struct {
	cfw    fs.CachedFileWatcher
	path   string
	logger *zap.Logger

	config *config.Config
	cbs    []WatcherCallback
}

func NewWatcher(cfw fs.CachedFileWatcher, path string, logger *zap.Logger) *Watcher {
	return &Watcher{
		cfw:    cfw,
		path:   path,
		logger: logger,
	}
}

func (cw *Watcher) AddCallback(cb WatcherCallback) {
	cw.cbs = append(cw.cbs, cb)
}

func (cw *Watcher) GetConfig() *config.Config {
	return cw.config
}

func (cw *Watcher) Start(ctx context.Context) error {
	err := cw.cfw.Add(cw.path, cw.update)
	if err != nil {
		return err
	}

	// Perform a first call to the callback with the contents of the config
	// file. Otherwise the callback won't be called until a modification
	// occurs.
	if cfg, err := cw.cfw.GetContent(cw.path); cfg != nil && err == nil {
		cw.update(cfg)
	}

	cw.cfw.Start(ctx)
	return nil
}

func (cw *Watcher) update(content []byte) {
	if len(content) == 0 {
		// Discard file events that do not inform content.
		cw.logger.Debug(fmt.Sprintf("Received event with empty contents for %s", cw.path))
		return
	}

	cfg, err := config.Parse(string(content))
	if err != nil {
		cw.logger.Error(fmt.Sprintf("Error parsing config from %s", cw.path), zap.Error(err))
		return
	}

	cw.config = cfg
	for _, cb := range cw.cbs {
		cb(cfg)
	}
}
