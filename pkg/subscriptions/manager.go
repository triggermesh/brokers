// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package subscriptions

import (
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"

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

	// Compare with existing
}

func (s *Manager) DispatchCloudEvent(event *cloudevents.Event) {
	s.logger.Info(fmt.Sprintf("Processing CloudEvent: %v", event))
	// TODO subscription management should ocurr here.
}
