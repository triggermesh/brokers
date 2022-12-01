// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package poller

import (
	"context"
	"fmt"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/triggermesh/brokers/pkg/common/fs"
	cfgbroker "github.com/triggermesh/brokers/pkg/config/broker"
)

type PollerCallback func(*cfgbroker.Config)

type Poller struct {
	fsp    fs.Poller
	path   string
	logger *zap.SugaredLogger

	config *cfgbroker.Config
	cbs    []PollerCallback
}

func NewPoller(fsp fs.Poller, path string, logger *zap.SugaredLogger) (*Poller, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("error resolving to absoluthe path %q: %w", path, err)
	}

	if absPath != path {
		return nil, fmt.Errorf("configuration path %q needs to be abstolute", path)
	}

	return &Poller{
		fsp:    fsp,
		path:   path,
		logger: logger,
	}, nil
}

func (cw *Poller) AddCallback(cb PollerCallback) {
	cw.cbs = append(cw.cbs, cb)
}

func (cw *Poller) GetConfig() *cfgbroker.Config {
	return cw.config
}

func (cw *Poller) Start(ctx context.Context) error {
	err := cw.fsp.Add(cw.path, cw.update)
	if err != nil {
		return err
	}

	if cfg, err := cw.fsp.GetContent(cw.path); cfg != nil && err == nil {
		cw.update(cfg)
	}

	cw.fsp.Start(ctx)
	return nil
}

func (cw *Poller) update(content []byte) {
	if len(content) == 0 {
		// Discard file events that do not inform content.
		cw.logger.Debug(fmt.Sprintf("Received event with empty contents for %s", cw.path))
		return
	}

	cfg, err := cfgbroker.Parse(string(content))
	if err != nil {
		cw.logger.Errorw(fmt.Sprintf("Error parsing config from %s", cw.path), zap.Error(err))
		return
	}

	cw.config = cfg
	for _, cb := range cw.cbs {
		cb(cfg)
	}
}
