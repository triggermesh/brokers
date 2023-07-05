// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package subscriptions

import (
	"context"
	"fmt"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/rickb777/date/period"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
	"knative.dev/pkg/logging"

	"github.com/triggermesh/brokers/pkg/backend"
	cfgbroker "github.com/triggermesh/brokers/pkg/config/broker"
	"github.com/triggermesh/brokers/pkg/status"
)

type subscriber struct {
	trigger cfgbroker.Trigger

	name          string
	backend       backend.Interface
	statusManager status.Manager
	ceClient      cloudevents.Client

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

func (s *subscriber) updateTrigger(trigger cfgbroker.Trigger) error {
	// Target URL might be informed as empty to support temporary
	// unavailability.
	url := ""
	if trigger.Target.URL != nil {
		url = *trigger.Target.URL
	}
	ctx := cloudevents.ContextWithTarget(s.parentCtx, url)

	// HACK temporary to make the Delivery options move smooth,
	// remove the method and access the field when the structure is
	// completely migrated to having the delivery options at the root.
	if do := trigger.GetDeliveryOptions(); do != nil &&
		do.Retry != nil &&
		*do.Retry >= 1 &&
		do.BackoffPolicy != nil {

		delay, err := period.Parse(*do.BackoffDelay)
		if err != nil {
			return fmt.Errorf("could not apply trigger %q configuration due to backoff delay parsing: %w", s.name, err)
		}

		switch *do.BackoffPolicy {
		case cfgbroker.BackoffPolicyLinear:
			ctx = cloudevents.ContextWithRetriesLinearBackoff(
				ctx, delay.DurationApprox(), int(*do.Retry))

		case cfgbroker.BackoffPolicyExponential:
			ctx = cloudevents.ContextWithRetriesExponentialBackoff(
				ctx, delay.DurationApprox(), int(*do.Retry))

		default:
			ctx = cloudevents.ContextWithRetriesConstantBackoff(
				ctx, delay.DurationApprox(), int(*do.Retry))
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

	if s.statusManager != nil {
		defer func() {
			t := time.Now()
			s.statusChange(&status.SubscriptionStatus{
				Status:        status.SubscriptionStatusRunning,
				LastProcessed: &t,
			})
		}()
	}

	res := subscriptionsapi.NewAllFilter(materializeFiltersList(s.ctx, s.trigger.Filters)...).Filter(s.ctx, *event)
	if res == eventfilter.FailFilter {
		s.logger.Debugw("Skipped delivery due to filter", zap.Any("event", *event))
		return
	}

	// Only try to send if target URL has been configured. When not
	// configured try to send to the dead letter sink.
	url := cloudevents.TargetFromContext(s.ctx)
	if url != nil && s.send(s.ctx, event) {
		return
	}

	// If the event could not be sent (including retries), check for DLS
	// and send if is it configured.
	if do := s.trigger.GetDeliveryOptions(); do != nil && do.DeadLetterURL != nil && *do.DeadLetterURL != "" {
		dlsCtx := cloudevents.ContextWithTarget(s.parentCtx, *do.DeadLetterURL)
		if s.send(dlsCtx, event) {
			return
		}
	}

	// If the event could not be sent either to the target or the DLS just write a log entry.
	// Set the attribute `lost: true` to help log aggregators identify lost events by querying.
	msg := "Event was lost"
	if url != nil {
		msg += " while sending to " + url.String()
	}
	s.logger.Errorw(msg, zap.Bool("lost", true),
		zap.String("type", event.Type()), zap.String("source", event.Source()), zap.String("id", event.ID()))
}

func (s *subscriber) statusChange(ss *status.SubscriptionStatus) {
	if s.statusManager != nil {
		s.statusManager.EnsureSubscription(s.name, ss)
	}
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

func materializeFiltersList(ctx context.Context, filters []cfgbroker.Filter) []eventfilter.Filter {
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

func materializeSubscriptionsAPIFilter(ctx context.Context, filter cfgbroker.Filter) eventfilter.Filter {
	var materializedFilter eventfilter.Filter
	var err error
	switch {
	case len(filter.Exact) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewExactFilter(filter.Exact)
		if err != nil {
			logging.FromContext(ctx).Debugw("Invalid exact expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.Prefix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewPrefixFilter(filter.Prefix)
		if err != nil {
			logging.FromContext(ctx).Debugw("Invalid prefix expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.Suffix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewSuffixFilter(filter.Suffix)
		if err != nil {
			logging.FromContext(ctx).Debugw("Invalid suffix expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.All) > 0:
		materializedFilter = subscriptionsapi.NewAllFilter(materializeFiltersList(ctx, filter.All)...)
	case len(filter.Any) > 0:
		materializedFilter = subscriptionsapi.NewAnyFilter(materializeFiltersList(ctx, filter.Any)...)
	case filter.Not != nil:
		materializedFilter = subscriptionsapi.NewNotFilter(materializeSubscriptionsAPIFilter(ctx, *filter.Not))
	}
	return materializedFilter
}
