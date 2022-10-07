// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package subscriptions

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	"knative.dev/pkg/logging"

	"github.com/triggermesh/brokers/pkg/backend"
	"github.com/triggermesh/brokers/pkg/config"
)

type Subscription struct {
	Trigger config.Trigger
}

type Manager struct {
	logger   *zap.SugaredLogger
	ceClient cloudevents.Client

	backend backend.Interface

	// Subscribers map indexed by name
	subscribers map[string]*subscriber

	ctx context.Context
	m   sync.RWMutex
}

func New(logger *zap.SugaredLogger, be backend.Interface) (*Manager, error) {
	// Needed for Knative filters
	ctx := context.Background()
	ctx = logging.WithLogger(ctx, logger)

	p, err := cloudevents.NewHTTP()
	if err != nil {
		return nil, fmt.Errorf("could not create CloudEvents HTTP protocol: %w", err)
	}

	ceClient, err := cloudevents.NewClient(p)
	if err != nil {
		return nil, fmt.Errorf("could not create CloudEvents HTTP client: %w", err)
	}

	return &Manager{
		backend:     be,
		subscribers: make(map[string]*subscriber),
		logger:      logger,
		ceClient:    ceClient,
		ctx:         ctx,
	}, nil
}

func (m *Manager) UpdateFromConfig(c *config.Config) {
	m.m.Lock()
	defer m.m.Unlock()

	for name, sub := range m.subscribers {
		if _, ok := c.Triggers[name]; !ok {
			m.logger.Info("Deleting subscription", zap.String("name", name))
			sub.unsubscribe()
			delete(m.subscribers, name)
		}
	}

	for name, trigger := range c.Triggers {
		s, ok := m.subscribers[name]
		if !ok {
			// If there is no subscription by that name, create one.
			s = &subscriber{
				name:      name,
				backend:   m.backend,
				ceClient:  m.ceClient,
				parentCtx: m.ctx,
				logger:    m.logger,
			}

			m.logger.Info("Creating new subscription from trigger configuration", zap.String("name", name), zap.Any("trigger", trigger))
			if err := s.updateTrigger(trigger); err != nil {
				m.logger.Error("Could not setup trigger", zap.String("trigger", name), zap.Error(err))
				continue
			}

			if err := m.backend.Subscribe(name, s.dispatchCloudEvent); err != nil {
				m.logger.Error("Could not create subscription for trigger", zap.String("trigger", name), zap.Error(err))
				continue
			}

			m.subscribers[name] = s
			continue
		}

		if reflect.DeepEqual(s.trigger, trigger) {
			// If there are no changes to the subscription, skip.
			continue
		}

		// Update existing subscription with new data.
		m.logger.Info("Updating subscription upon trigger configuration", zap.String("name", name), zap.Any("trigger", trigger))
		if err := s.updateTrigger(trigger); err != nil {
			m.logger.Error("Could not setup trigger", zap.String("name", name), zap.Error(err))
			return
		}
	}
}
