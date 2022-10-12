// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package lib

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type Producer interface {
	Produce(ctx context.Context, event cloudevents.Event) error
	GetStoredEvents() []StoredEvent
}

type SimpleProducer struct {
	endpoint string
	store    Store
}

func NewSimpleProducer(endpoint string) Producer {
	return &SimpleProducer{
		endpoint: endpoint,
	}
}

func (p *SimpleProducer) Produce(ctx context.Context, event cloudevents.Event) error {
	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		return err
	}

	result := c.Send(cloudevents.ContextWithTarget(ctx, p.endpoint), event)

	p.store.Add(event, StoredEventWithResult(result))
	if cloudevents.IsUndelivered(result) {
		return result
	}

	return nil
}

func (p *SimpleProducer) GetStoredEvents() []StoredEvent {
	return p.store.elements
}
