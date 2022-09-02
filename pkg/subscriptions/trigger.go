// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package subscriptions

import (
	"github.com/triggermesh/brokers/pkg/config"
	"go.uber.org/zap"
)

type Manager struct {
	logger *zap.Logger
}

func New(logger *zap.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

func (s *Manager) UpdateFromConfig(c *config.Config) {
	s.logger.Info("Subscription Manager UpdateFromConfig ...")
}
