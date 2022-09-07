// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"errors"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	"github.com/triggermesh/brokers/pkg/backend"
)

func New(args *MemoryArgs, logger *zap.Logger) backend.Interface {
	return &memory{
		closing: false,
		args:    args,
		logger:  logger,
	}
}

type memory struct {
	args *MemoryArgs

	closing bool
	buffer  chan *cloudevents.Event
	logger  *zap.Logger
}

func (s *memory) Info() *backend.Info {
	return &backend.Info{
		Name: "Memory",
	}
}

func (s *memory) Init(ctx context.Context) error {
	s.buffer = make(chan *cloudevents.Event, s.args.BufferSize)
	return nil
}

func (s *memory) Disconnect() error {
	return nil
}

func (s *memory) Produce(ctx context.Context, event *cloudevents.Event) error {
	if s.closing {
		return errors.New("rejecting events due to backend closing")
	}

	select {
	case <-time.After(s.args.ProduceTimeout):
		return fmt.Errorf("failed to add the event to the buffer after %d", s.args.ProduceTimeout)
	case s.buffer <- event:
	}
	return nil
}

func (s *memory) Start(ctx context.Context, ccb backend.ConsumerDispatcher) {
	go func() {
		<-ctx.Done()
		s.closing = true
	}()

	for {
		select {
		case event := <-s.buffer:
			ccb(event)
		case <-ctx.Done():
			// signal to reject new events being produced
			s.closing = true
			close(s.buffer)

			// loop all remaining elements from the channel
			for event := range s.buffer {
				ccb(event)
			}
		}

		if s.closing {
			break
		}
	}
}

func (s *memory) Probe(ctx context.Context) error {
	return nil
}
