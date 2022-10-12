// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package lib

import (
	"context"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type Consumer interface {
	Start(ctx context.Context) error
	GetStoredEvents() []StoredEvent
	GetConsumerEndPoint() string
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
	return fmt.Sprintf("0.0.0.0:%d", s.port)
}
