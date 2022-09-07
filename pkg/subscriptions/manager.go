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

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
	"knative.dev/pkg/logging"

	"github.com/triggermesh/brokers/pkg/config"
)

type Manager struct {
	logger   *zap.Logger
	ceClient cloudevents.Client

	triggers []config.Trigger
	ctx      context.Context
	m        sync.RWMutex
}

func New(logger *zap.Logger) (*Manager, error) {
	// Needed for Knative filters
	ctx := context.Background()
	ctx = logging.WithLogger(ctx, logger.Sugar())

	p, err := cloudevents.NewHTTP()
	if err != nil {
		return nil, fmt.Errorf("Could not create CloudEvents HTTP protocol: %w", err)
	}

	ceClient, err := cloudevents.NewClient(p)
	if err != nil {
		return nil, fmt.Errorf("Could not create CloudEvents HTTP client: %w", err)
	}

	return &Manager{
		logger:   logger,
		ceClient: ceClient,
		ctx:      ctx,
	}, nil
}

func (m *Manager) UpdateFromConfig(c *config.Config) {
	m.m.Lock()
	defer m.m.Unlock()

	if reflect.DeepEqual(m.triggers, c.Triggers) {
		return
	}

	m.logger.Info("Updating trigger configuration", zap.Any("triggers", c.Triggers))
	m.triggers = c.Triggers
}

func (m *Manager) DispatchCloudEvent(event *cloudevents.Event) {
	m.logger.Info(fmt.Sprintf("Processing CloudEvent: %v", event))

	m.m.RLock()
	defer m.m.RUnlock()

	var wg sync.WaitGroup
	for i := range m.triggers {
		res := subscriptionsapi.NewAllFilter(materializeFiltersList(m.ctx, m.triggers[i].Filters)...).Filter(m.ctx, *event)
		if res == eventfilter.FailFilter {
			m.logger.Debug("Skipped delivery due to filter", zap.Any("event", *event))
			return
		}

		for j := range m.triggers[i].Targets {
			target := &m.triggers[i].Targets[j]
			wg.Add(1)
			go func() {
				defer wg.Done()
				m.DispatchCloudEventToTarget(target, event)
			}()
		}
	}
	wg.Wait()
}

func (m *Manager) DispatchCloudEventToTarget(target *config.Target, event *cloudevents.Event) {
	ctx := cloudevents.ContextWithTarget(m.ctx, target.URL)

	if target.DeliveryOptions != nil && target.DeliveryOptions.Retries > 1 {

		switch target.DeliveryOptions.BackoffPolicy {
		case config.BackoffPolicyLinear:
			ctx = cloudevents.ContextWithRetriesLinearBackoff(
				ctx, target.DeliveryOptions.BackoffDelay, target.DeliveryOptions.Retries)

		case config.BackoffPolicyExponential:
			ctx = cloudevents.ContextWithRetriesExponentialBackoff(
				ctx, target.DeliveryOptions.BackoffDelay, target.DeliveryOptions.Retries)

		default:
			ctx = cloudevents.ContextWithRetriesConstantBackoff(
				ctx, target.DeliveryOptions.BackoffDelay, target.DeliveryOptions.Retries)
		}
	}

	result := m.ceClient.Send(ctx, *event)
	dls := false
	switch {
	case cloudevents.IsACK(result):
		return

	case cloudevents.IsUndelivered(result):
		m.logger.Error(fmt.Sprintf("Failed to send event to %s", target.URL),
			zap.Error(result), zap.String("type", event.Type()), zap.String("source", event.Source()), zap.String("id", event.ID()))
		dls = true

	case cloudevents.IsNACK(result):
		m.logger.Error(fmt.Sprintf("Event not accepted at %s", target.URL),
			zap.Error(result), zap.String("type", event.Type()), zap.String("source", event.Source()), zap.String("id", event.ID()))
		dls = true
	}

	// TODO check if dls is enabled for the target
	if dls {
		// TODO send to DLS
		m.logger.Info("NEED TO SEND TO DLS", zap.Any("event", *event))
	}
}

// Copied from Knative Eventing

/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

func materializeFiltersList(ctx context.Context, filters []eventingv1.SubscriptionsAPIFilter) []eventfilter.Filter {
	materializedFilters := make([]eventfilter.Filter, 0, len(filters))
	for _, f := range filters {
		f := materializeSubscriptionsAPIFilter(ctx, f)
		if f == nil {
			logging.FromContext(ctx).Warnw("Failed to parse filter. Skipping filter.", zap.Any("filter", f))
			continue
		}
		materializedFilters = append(materializedFilters, f)
	}
	return materializedFilters
}

func materializeSubscriptionsAPIFilter(ctx context.Context, filter eventingv1.SubscriptionsAPIFilter) eventfilter.Filter {
	var materializedFilter eventfilter.Filter
	var err error
	switch {
	case len(filter.Exact) > 0:
		// The webhook validates that this map has only a single key:value pair.
		for attribute, value := range filter.Exact {
			materializedFilter, err = subscriptionsapi.NewExactFilter(attribute, value)
			if err != nil {
				logging.FromContext(ctx).Debugw("Invalid exact expression", zap.String("attribute", attribute), zap.String("value", value), zap.Error(err))
				return nil
			}
		}
	case len(filter.Prefix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		for attribute, prefix := range filter.Prefix {
			materializedFilter, err = subscriptionsapi.NewPrefixFilter(attribute, prefix)
			if err != nil {
				logging.FromContext(ctx).Debugw("Invalid prefix expression", zap.String("attribute", attribute), zap.String("prefix", prefix), zap.Error(err))
				return nil
			}
		}
	case len(filter.Suffix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		for attribute, suffix := range filter.Suffix {
			materializedFilter, err = subscriptionsapi.NewSuffixFilter(attribute, suffix)
			if err != nil {
				logging.FromContext(ctx).Debugw("Invalid suffix expression", zap.String("attribute", attribute), zap.String("suffix", suffix), zap.Error(err))
				return nil
			}
		}
	case len(filter.All) > 0:
		materializedFilter = subscriptionsapi.NewAllFilter(materializeFiltersList(ctx, filter.All)...)
	case len(filter.Any) > 0:
		materializedFilter = subscriptionsapi.NewAnyFilter(materializeFiltersList(ctx, filter.Any)...)
	case filter.Not != nil:
		materializedFilter = subscriptionsapi.NewNotFilter(materializeSubscriptionsAPIFilter(ctx, *filter.Not))
	case filter.CESQL != "":
		if materializedFilter, err = subscriptionsapi.NewCESQLFilter(filter.CESQL); err != nil {
			// This is weird, CESQL expression should be validated when Trigger's are created.
			logging.FromContext(ctx).Debugw("Found an Invalid CE SQL expression", zap.String("expression", filter.CESQL))
			return nil
		}
	}
	return materializedFilter
}
