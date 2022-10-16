// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"fmt"
	"path/filepath"

	"go.uber.org/zap"
	"sigs.k8s.io/yaml"

	"github.com/triggermesh/brokers/pkg/common/fs"
	"github.com/triggermesh/brokers/pkg/config/observability"
)

type WatcherCallback func(*observability.Config)

type Watcher struct {
	cfw    fs.CachedFileWatcher
	path   string
	logger *zap.SugaredLogger

	config *observability.Config
	cbs    []WatcherCallback
}

func NewWatcher(cfw fs.CachedFileWatcher, path string, logger *zap.SugaredLogger) (*Watcher, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("error resolving to absoluthe path %q: %w", path, err)
	}

	if absPath != path {
		return nil, fmt.Errorf("configuration path %q needs to be abstolute", path)
	}

	return &Watcher{
		cfw:    cfw,
		path:   path,
		logger: logger,
	}, nil
}

func (cw *Watcher) AddCallback(cb WatcherCallback) {
	cw.cbs = append(cw.cbs, cb)
}

func (cw *Watcher) GetConfig() *observability.Config {
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
		return
	}

	cfg := &observability.Config{}
	if err := yaml.Unmarshal([]byte(content), cfg); err != nil {
		cw.logger.Errorw(fmt.Sprintf("Contents for %s are not valid", cw.path), zap.Error(err))
		return
	}

	cw.config = cfg
	for _, cb := range cw.cbs {
		cb(cfg)
	}
}
