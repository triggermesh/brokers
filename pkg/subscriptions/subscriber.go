// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package subscriptions

import (
	"context"
	"fmt"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/rickb777/date/period"
	"go.uber.org/zap"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
	"knative.dev/pkg/logging"

	"github.com/triggermesh/brokers/pkg/backend"
	"github.com/triggermesh/brokers/pkg/config"
)

type subscriber struct {
	trigger config.Trigger

	name     string
	backend  backend.Interface
	ceClient cloudevents.Client

	// We need to have both the parent context used to build the subscriber and the
	// local context used to send CloudEvents that contains the target and delivery
	// options.
	// The local context will be re-created from the parent context every time a
	// change is done to the trigger
	parentCtx context.Context
	ctx       context.Context

	logger *zap.SugaredLogger
	m      sync.RWMutex
}

func (s *subscriber) unsubscribe() {
	s.backend.Unsubscribe(s.name)
}

func (s *subscriber) updateTrigger(trigger config.Trigger) error {
	ctx := cloudevents.ContextWithTarget(s.parentCtx, trigger.Target.URL)
	if trigger.Target.DeliveryOptions != nil &&
		trigger.Target.DeliveryOptions.Retry != nil &&
		*trigger.Target.DeliveryOptions.Retry >= 1 &&
		trigger.Target.DeliveryOptions.BackoffPolicy != nil {

		delay, err := period.Parse(*trigger.Target.DeliveryOptions.BackoffDelay)
		if err != nil {
			return fmt.Errorf("could not apply trigger %q configuration due to backoff delay parsing: %v", s.name, err)
		}

		switch *trigger.Target.DeliveryOptions.BackoffPolicy {
		case config.BackoffPolicyLinear:
			ctx = cloudevents.ContextWithRetriesLinearBackoff(
				ctx, delay.DurationApprox(), int(*trigger.Target.DeliveryOptions.Retry))

		case config.BackoffPolicyExponential:
			ctx = cloudevents.ContextWithRetriesExponentialBackoff(
				ctx, delay.DurationApprox(), int(*trigger.Target.DeliveryOptions.Retry))

		default:
			ctx = cloudevents.ContextWithRetriesConstantBackoff(
				ctx, delay.DurationApprox(), int(*trigger.Target.DeliveryOptions.Retry))
		}
	}

	s.m.Lock()
	defer s.m.Unlock()

	s.trigger = trigger
	s.ctx = ctx

	return nil
}

func (s *subscriber) dispatchCloudEvent(event *cloudevents.Event) {
	s.m.RLock()
	defer s.m.RUnlock()

	res := subscriptionsapi.NewAllFilter(materializeFiltersList(s.ctx, s.trigger.Filters)...).Filter(s.ctx, *event)
	if res == eventfilter.FailFilter {
		s.logger.Debug("Skipped delivery due to filter", zap.Any("event", *event))
		return
	}

	t := s.trigger.Target
	s.dispatchCloudEventToTarget(&t, event)
}

func (s *subscriber) dispatchCloudEventToTarget(target *config.Target, event *cloudevents.Event) {
	if s.send(s.ctx, event) {
		return
	}

	if target.DeliveryOptions != nil && target.DeliveryOptions.DeadLetterURL != nil &&
		*target.DeliveryOptions.DeadLetterURL != "" {
		dlsCtx := cloudevents.ContextWithTarget(s.parentCtx, *target.DeliveryOptions.DeadLetterURL)
		if s.send(dlsCtx, event) {
			return
		}
	}

	// Attribute "lost": true is set help log aggregators identify
	// lost events by querying.
	s.logger.Errorw(fmt.Sprintf("Event was lost while sending to %s",
		cloudevents.TargetFromContext(s.ctx).String()), zap.Bool("lost", true),
		zap.String("type", event.Type()), zap.String("source", event.Source()), zap.String("id", event.ID()))
}

func (s *subscriber) send(ctx context.Context, event *cloudevents.Event) bool {
	res, result := s.ceClient.Request(ctx, *event)

	switch {
	case cloudevents.IsACK(result):
		if res != nil {
			if err := s.backend.Produce(ctx, res); err != nil {
				s.logger.Errorw(fmt.Sprintf("Failed to consume response from %s",
					cloudevents.TargetFromContext(ctx).String()),
					zap.Error(err), zap.String("type", res.Type()), zap.String("source", res.Source()), zap.String("id", res.ID()))

				// Not ingesting the response is considered an error.
				// TODO make this configurable.
				return false
			}
		}
		return true

	case cloudevents.IsUndelivered(result):
		s.logger.Errorw(fmt.Sprintf("Failed to send event to %s",
			cloudevents.TargetFromContext(ctx).String()),
			zap.Error(result), zap.String("type", event.Type()), zap.String("source", event.Source()), zap.String("id", event.ID()))
		return false

	case cloudevents.IsNACK(result):
		s.logger.Errorw(fmt.Sprintf("Event not accepted at %s",
			cloudevents.TargetFromContext(ctx).String()),
			zap.Error(result), zap.String("type", event.Type()), zap.String("source", event.Source()), zap.String("id", event.ID()))
		return false
	}

	s.logger.Errorw(fmt.Sprintf("Unknown event send outcome at %s",
		cloudevents.TargetFromContext(ctx).String()),
		zap.Error(result), zap.String("type", event.Type()), zap.String("source", event.Source()), zap.String("id", event.ID()))
	return false
}

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
