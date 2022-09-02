// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
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

type Interface interface {
	// Info returns information about the backend implementation.
	Info() *Info

	// Init connects and does initialization tasks at the backend.
	// It must be called before using other methods (but Info)
	// and might perform initialization tasks like creating structures
	// or migrations.
	Init(ctx context.Context) error

	// Produce new CloudEvents to the backend.
	Produce(context.Context, *cloudevents.Event) error

	// Start is a blocking method that read events from the backend
	// and pass them to the consumer dispatcher. When the consumer
	// dispatcher returns, the message is marked as processed and
	// won't be delivered anymore.
	Start(context.Context, ConsumerDispatcher)

	// Probe checks the overall status of the backend implementation.
	Probe(context.Context) error

	// Disconnect is a blocking method that makes sure that on the fly
	// tasks are finished and the backend is left in a clean state.
	Disconnect() error
}
