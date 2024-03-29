// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"github.com/triggermesh/brokers/pkg/backend"
	"github.com/triggermesh/brokers/pkg/status"
)

const (
	BackendIDAttribute = "triggermeshbackendid"
)

type exceedBounds func(r *kgo.Record) bool

type endBound struct {
	id   int64
	time *time.Time
}

func newExceedBounds(eb *endBound) exceedBounds {
	switch {
	case eb == nil:
		return nil

	case eb.id == 0 && eb.time == nil:
		return nil

	case eb.id == 0:
		return func(r *kgo.Record) bool {
			return r.Timestamp.After(*eb.time)
		}

	case eb.time == nil:
		return func(r *kgo.Record) bool {
			return r.Offset >= eb.id
		}

	default:
		return func(r *kgo.Record) bool {
			return r.Offset >= eb.id || r.Timestamp.After(*eb.time)
		}
	}
}

type subscription struct {
	instance string
	topic    string
	// name     string
	group               string
	checkBoundsExceeded exceedBounds

	trackingEnabled bool

	// caller's callback for dispatching events from Redis.
	ccbDispatch backend.ConsumerDispatcher

	// caller's callback for subscription status changes
	scb backend.SubscriptionStatusChange

	// cancel function let us control when the subscription loop should exit.
	ctx    context.Context
	cancel context.CancelFunc
	// stoppedCh signals when a subscription has completely finished.
	stoppedCh chan struct{}

	client *kgo.Client
	logger *zap.SugaredLogger
}

func (s *subscription) start() {
	s.logger.Infow("Starting Kafka consumer",
		zap.String("group", s.group),
		zap.String("instance", s.instance),
		zap.String("topic", s.topic))
	// Start reading all pending messages
	// id := "0"

	// When the context is signaled mark an exitLoop flag to exit
	// the worker routine gracefuly.
	exitLoop := false
	go func() {
		<-s.ctx.Done()

		s.logger.Debugw("Waiting for last fetch operation to finish before exiting subscription",
			zap.String("group", s.group),
			zap.String("instance", s.instance),
			zap.String("topic", s.topic))
		exitLoop = true
	}()

	go func() {
		for {
			// Check at the begining of each iteration if the exit loop flag has
			// been signaled due to done context or because the endDate has been reached.
			if exitLoop {
				break
			}

			// Although this call is blocking it will yield when the context is done,
			// the exit loop flag above will be triggered almost immediately if no
			// data has been read.
			fetches := s.client.PollFetches(s.ctx)
			if fetches.IsClientClosed() {
				// Let's assume we closed the client and exit the loop
				s.logger.Warn("Exiting consumer due to client closed", zap.String("group", s.group))
				break
			}

			fetches.EachError(func(_ string, p int32, err error) {
				// TODO: we could refine errors and avoid exiting
				// under all error conditions. There might be some recoverable
				// errors that do not require exiting.
				if !exitLoop {
					exitLoop = true
				}
				s.logger.Error("Event consumption error",
					zap.String("group", s.group), zap.Int32("partition", p), zap.Error(err))
			})

			fetches.EachRecord(func(record *kgo.Record) {
				defer func() {
					if err := s.client.CommitUncommittedOffsets(s.ctx); err != nil {
						s.logger.Errorw("Could not commit offsets", zap.Error(err))
					}
				}()

				ce := &cloudevents.Event{}
				if err := ce.UnmarshalJSON(record.Value); err != nil {
					s.logger.Errorw("Could not unmarshal CloudEvent from Kafka", zap.Error(err))
					return
				}

				// If there was no valid CE in the message ACK so that we do not receive it again.
				if err := ce.Validate(); err != nil {
					s.logger.Warn(fmt.Sprintf("Removing non CloudEvent message from backend: %v", record.Offset))
					return
				}

				// // If an end date has been specified, compare the current message ID
				// // with the end date. If the message ID is newer than the end date,
				// // exit the loop.
				if s.checkBoundsExceeded != nil {
					if exitLoop = s.checkBoundsExceeded(record); exitLoop {
						s.scb(&status.SubscriptionStatus{
							Status: status.SubscriptionStatusComplete,
						})
						return
					}
				}

				if s.trackingEnabled {
					if err := ce.Context.SetExtension(BackendIDAttribute, record.Offset); err != nil {
						s.logger.Errorw(fmt.Sprintf("could not set %s attributes for the Kafka offset %d. Tracking will not be possible.", BackendIDAttribute, record.Offset),
							zap.Error(err))
					}
				}

				go func(rs *kgo.Record) {
					s.ccbDispatch(ce)
					if err := s.client.CommitRecords(s.ctx, rs); err != nil {
						s.logger.Errorw(fmt.Sprintf("could not commit the Kafka offset %d containing CloudEvent %s", rs.Offset, ce.Context.GetID()),
							zap.Error(err))
					}
				}(record)
			})
		}

		s.logger.Debugw("Exited Kafka subscription",
			zap.String("group", s.group),
			zap.String("instance", s.instance),
			zap.String("topic", s.group))

		// Close stoppedCh to signal external viewers that processing for this
		// subscription is no longer running.
		close(s.stoppedCh)

	}()
}
