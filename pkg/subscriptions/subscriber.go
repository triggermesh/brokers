package subscriptions

import (
	"context"
	"fmt"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/rickb777/date/period"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"

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

// func newSubscriber(ctx context.Context, name string, backend backend.Interface, ceClient cloudevents.Client, trigger config.Trigger, logger *zap.SugaredLogger) *subscriber {
// 	// s := &subscriber{
// 	// 	backend:  backend,
// 	// 	ceClient: ceClient,
// 	// 	logger:   logger,
// 	// }

// 	s := &subscriber{
// 		name:      name,
// 		backend:   backend,
// 		ceClient:  ceClient,
// 		parentCtx: ctx,
// 		logger:    logger,
// 	}

// 	if err := s.updateTrigger(trigger); err != nil {
// 		m.logger.Error("Could not setup trigger", zap.String("trigger", name), zap.Error(err))
// 		return
// 	}

// 	backend.Subscribe(name, s.dispatchCloudEvent)

// 	return s
// }

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
	s.logger.Error(fmt.Sprintf("Event was lost while sending to %s",
		cloudevents.TargetFromContext(s.ctx).String()), zap.Bool("lost", true),
		zap.String("type", event.Type()), zap.String("source", event.Source()), zap.String("id", event.ID()))
}

func (s *subscriber) send(ctx context.Context, event *cloudevents.Event) bool {
	res, result := s.ceClient.Request(ctx, *event)

	switch {
	case cloudevents.IsACK(result):
		if res != nil {
			if err := s.backend.Produce(ctx, res); err != nil {
				s.logger.Error(fmt.Sprintf("Failed to consume response from %s",
					cloudevents.TargetFromContext(ctx).String()),
					zap.Error(err), zap.String("type", res.Type()), zap.String("source", res.Source()), zap.String("id", res.ID()))

				// Not ingesting the response is considered an error.
				// TODO make this configurable.
				return false
			}
		}
		return true

	case cloudevents.IsUndelivered(result):
		s.logger.Error(fmt.Sprintf("Failed to send event to %s",
			cloudevents.TargetFromContext(ctx).String()),
			zap.Error(result), zap.String("type", event.Type()), zap.String("source", event.Source()), zap.String("id", event.ID()))
		return false

	case cloudevents.IsNACK(result):
		s.logger.Error(fmt.Sprintf("Event not accepted at %s",
			cloudevents.TargetFromContext(ctx).String()),
			zap.Error(result), zap.String("type", event.Type()), zap.String("source", event.Source()), zap.String("id", event.ID()))
		return false
	}

	s.logger.Error(fmt.Sprintf("Unknown event send outcome at %s",
		cloudevents.TargetFromContext(ctx).String()),
		zap.Error(result), zap.String("type", event.Type()), zap.String("source", event.Source()), zap.String("id", event.ID()))
	return false
}
