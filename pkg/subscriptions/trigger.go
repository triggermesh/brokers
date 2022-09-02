// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package subscriptions

import (
	"github.com/triggermesh/brokers/pkg/backend"
	"github.com/triggermesh/brokers/pkg/config"
	"go.uber.org/zap"
)

type Manager struct {
	backend backend.Interface

	logger *zap.Logger
}

func New(backend backend.Interface, logger *zap.Logger) *Manager {
	return &Manager{
		backend: backend,
		logger:  logger,
	}
}

func (s *Manager) UpdateFromConfig(c *config.Config) {
	s.logger.Info("Subscription Manager UpdateFromConfig ...")
}
