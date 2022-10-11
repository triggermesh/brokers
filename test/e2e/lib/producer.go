//go:build e2e
// +build e2e

package lib

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type Producer struct {
	endpoint string
}

func NewProducer(endpoint string) *Producer {
	return &Producer{
		endpoint: endpoint,
	}
}

func (p *Producer) Produce(ctx context.Context, event cloudevents.Event) error {
	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		return err
	}

	if result := c.Send(cloudevents.ContextWithTarget(ctx, p.endpoint), event); cloudevents.IsUndelivered(result) {
		return result
	}

	return nil
}
