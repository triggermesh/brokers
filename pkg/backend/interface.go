// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/triggermesh/brokers/pkg/config/broker"
	"github.com/triggermesh/brokers/pkg/status"
)

type Info struct {
	// Name of the backend implementation
	Name string
}

// ConsumerDispatcher receives CloudEvents to be delivered to subscribers.
// The consumer dispatcher must process the event for all subscriptions,
// including retries and dead leter queues.
// When the function finishes executing the backend will consider
// the event processed and will make sure it is not re-delivered.
type ConsumerDispatcher func(event *cloudevents.Event)

type SubscriptionStatusChange func(*status.SubscriptionStatus)

type EventProducer interface {
	// Ingest a new CloudEvents at the backend.
	Produce(context.Context, *cloudevents.Event) error
}

type SubscribeOption func(Subscribable)

type Subscribable interface {
	// Subscribe is a method that sets up a reader that will retrieve
	// events from the backend and pass them to the consumer dispatcher.
	// When the consumer dispatcher returns, the message is marked as
	// processed and won't be delivered anymore.
	Subscribe(name string, bounds *broker.TriggerBounds, ccb ConsumerDispatcher, scb SubscriptionStatusChange) error

	// Unsubscribe is a method that removes a subscription referencing
	// it by name, returning when all pending (already read) messages
	// have been dispatched.
	Unsubscribe(name string)
}

type Interface interface {
	EventProducer
	Subscribable

	// Info returns information about the backend implementation.
	Info() *Info

	// Init connects and does initialization tasks at the backend.
	// It must be called before using other methods (but Info)
	// and might perform initialization tasks like creating structures
	// or migrations.
	Init(ctx context.Context) error

	// Start is a blocking method that read events from the backend
	// and pass them to the subscriber's consumer dispatcher. When the consumer
	// dispatcher returns, the message is marked as processed and
	// won't be delivered anymore.
	// When the context is done all subscribers are finished and the
	// method exists.
	Start(ctx context.Context) error

	// Probe checks the overall status of the backend implementation.
	Probe(context.Context) error
}
