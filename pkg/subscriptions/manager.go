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

type CloudEventHandler func(context.Context, *cloudevents.Event) error

type Subscription struct {
	Trigger config.Trigger
}

type Manager struct {
	logger   *zap.SugaredLogger
	ceClient cloudevents.Client

	backend backend.Interface

	subscribers map[string]*subscriber

	// TODO subs map

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

	for k, sub := range m.subscribers {
		if _, ok := c.Triggers[k]; !ok {
			sub.unsubscribe()
			delete(m.subscribers, k)
		}
	}

	for name, trigger := range c.Triggers {
		s, ok := m.subscribers[name]
		if !ok {
			// if not exists create subscription.
			s = &subscriber{
				name:      name,
				backend:   m.backend,
				ceClient:  m.ceClient,
				parentCtx: m.ctx,
				logger:    m.logger,
			}

			if err := s.updateTrigger(trigger); err != nil {
				m.logger.Error("Could not setup trigger", zap.String("trigger", name), zap.Error(err))
				return
			}

			m.backend.Subscribe(name, s.dispatchCloudEvent)
			m.subscribers[name] = s

			continue
		}

		if reflect.DeepEqual(s.trigger, trigger) {
			// no changes for this trigger.
			continue
		}

		// if exists, update data
		m.logger.Info("Updating trigger configuration", zap.String("name", name), zap.Any("trigger", trigger))
		if err := s.updateTrigger(trigger); err != nil {
			m.logger.Error("Could not setup trigger", zap.String("name", name), zap.Error(err))
			return
		}
	}
}
