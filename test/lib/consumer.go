// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package lib

import (
	"context"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type Consumer interface {
	Start(ctx context.Context) error
	GetStoredEvents() []StoredEvent
	GetConsumerEndPoint() string
	WaitForEvent(timeout time.Duration, filters ...EventFilterOption) bool
}

type SimpleConsumer struct {
	client cloudevents.Client
	store  Store
	port   int
}

func NewSimpleConsumer(port int) Consumer {
	client, err := cloudevents.NewClientHTTP(
		cloudevents.WithPort(port),
	)
	if err != nil {
		// chances of erroring creating the CloudEvents client
		// are low, and returning an error spoils the simplicity.
		panic(err)
	}

	return &SimpleConsumer{
		client: client,
		port:   port,
	}
}

func (s *SimpleConsumer) Start(ctx context.Context) error {
	return s.client.StartReceiver(ctx, s.receive)
}

func (s *SimpleConsumer) receive(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, cloudevents.Result) {
	s.store.Add(event, StoredEventWithResult(cloudevents.ResultACK))
	return nil, cloudevents.ResultACK
}

func (s *SimpleConsumer) GetStoredEvents() []StoredEvent {
	return s.store.elements
}

func (s *SimpleConsumer) GetConsumerEndPoint() string {
	return fmt.Sprintf("http://0.0.0.0:%d", s.port)
}

type EventFilterOption func(event *cloudevents.Event) bool

func EventiFilterWithID(id string) EventFilterOption {
	return func(event *cloudevents.Event) bool {
		return event.ID() == id
	}
}

func (s *SimpleConsumer) WaitForEvent(timeout time.Duration, filters ...EventFilterOption) bool {
	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, e := range s.store.elements {

				valid := true
				for _, filter := range filters {
					if !filter(&e.Event) {
						valid = false
						break
					}
				}

				if valid {
					return true
				}
			}

		case <-timeoutCh:
			return false
		}
	}
}
