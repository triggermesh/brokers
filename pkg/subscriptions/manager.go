// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package subscriptions

import (
	"context"
	"reflect"
	"sync"

	obshttp "github.com/cloudevents/sdk-go/observability/opencensus/v2/http"
	ceclient "github.com/cloudevents/sdk-go/v2/client"
	"go.uber.org/zap"

	"knative.dev/pkg/logging"

	"github.com/triggermesh/brokers/pkg/backend"
	cfgbroker "github.com/triggermesh/brokers/pkg/config/broker"
	"github.com/triggermesh/brokers/pkg/status"
	"github.com/triggermesh/brokers/pkg/subscriptions/metrics"
)

type Subscription struct {
	Trigger cfgbroker.Trigger
}

type Manager struct {
	logger *zap.SugaredLogger

	backend       backend.Interface
	statusManager status.Manager

	// Subscribers map indexed by name
	subscribers map[string]*subscriber

	ctx context.Context
	m   sync.RWMutex
}

func New(inctx context.Context, logger *zap.SugaredLogger, be backend.Interface, statusManager status.Manager) (*Manager, error) {
	// Needed for Knative filters
	ctx := logging.WithLogger(inctx, logger)

	return &Manager{
		backend:       be,
		subscribers:   make(map[string]*subscriber),
		logger:        logger,
		statusManager: statusManager,
		ctx:           ctx,
	}, nil
}

func (m *Manager) UpdateFromConfig(c *cfgbroker.Config) {
	m.logger.Info("Updating subscriptions configuration")
	m.m.Lock()
	defer m.m.Unlock()

	for name, sub := range m.subscribers {
		if _, ok := c.Triggers[name]; !ok {
			m.logger.Infow("Deleting subscription", zap.String("name", name))
			sub.unsubscribe()
			delete(m.subscribers, name)
		}
	}

	for name, trigger := range c.Triggers {
		s, ok := m.subscribers[name]
		if !ok {
			s := m.createSubscriber(name, trigger)
			if s == nil {
				continue
			}
			m.subscribers[name] = s
			m.logger.Infow("Subscription for trigger updated", zap.String("name", name))
			continue
		}

		if reflect.DeepEqual(s.trigger, trigger) {
			// If there are no changes to the subscription, skip.
			continue
		}

		// Update existing subscription with new data.
		m.logger.Infow("Updating subscription upon trigger configuration", zap.String("name", name), zap.Any("trigger", trigger))
		if err := s.updateTrigger(trigger); err != nil {
			m.logger.Errorw("Could not setup trigger", zap.String("name", name), zap.Error(err))
			return
		}
	}
}

func (m *Manager) createSubscriber(name string, trigger cfgbroker.Trigger) *subscriber {
	// Create CloudEvents client with reporter for Trigger.
	ir, err := metrics.NewReporter(m.ctx, name)
	if err != nil {
		m.logger.Errorw("Failed to setup trigger stats reporter", zap.String("trigger", name), zap.Error(err))
		return nil
	}

	p, err := obshttp.NewObservedHTTP()
	if err != nil {
		m.logger.Errorw("Could not create CloudEvents HTTP protocol", zap.String("trigger", name), zap.Error(err))
		return nil
	}

	ceClient, err := ceclient.New(p, ceclient.WithObservabilityService(metrics.NewOpenCensusObservabilityService(ir)))
	if err != nil {
		m.logger.Errorw("Could not create CloudEvents HTTP client", zap.String("trigger", name), zap.Error(err))
		return nil
	}

	s := &subscriber{
		name:          name,
		backend:       m.backend,
		statusManager: m.statusManager,
		ceClient:      ceClient,
		parentCtx:     m.ctx,
		logger:        m.logger,
	}

	m.logger.Infow("Creating new subscription from trigger configuration", zap.String("name", name), zap.Any("trigger", trigger))
	if err := s.updateTrigger(trigger); err != nil {
		m.logger.Errorw("Could not setup trigger", zap.String("trigger", name), zap.Error(err))
		return nil
	}

	if err := m.backend.Subscribe(name, trigger.Bounds, s.dispatchCloudEvent); err != nil {
		m.logger.Errorw("Could not create subscription for trigger", zap.String("trigger", name), zap.Error(err))
		return nil
	}

	return s
}
