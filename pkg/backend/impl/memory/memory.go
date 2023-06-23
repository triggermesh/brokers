// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	"github.com/triggermesh/brokers/pkg/backend"
	"github.com/triggermesh/brokers/pkg/config/broker"
)

func New(args *MemoryArgs, logger *zap.SugaredLogger) backend.Interface {
	return &memory{
		ccbs:    make(map[string]backend.ConsumerDispatcher),
		closing: false,
		args:    args,
		logger:  logger,
	}
}

type memory struct {
	args *MemoryArgs

	ccbs    map[string]backend.ConsumerDispatcher
	closing bool
	buffer  chan *cloudevents.Event
	logger  *zap.SugaredLogger
	m       sync.RWMutex
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

func (s *memory) Produce(ctx context.Context, event *cloudevents.Event) error {
	if s.closing {
		return errors.New("rejecting events due to backend closing")
	}

	select {
	case <-time.After(s.args.ProduceTimeoutDuration):
		return fmt.Errorf("failed to add the event to the buffer after %s", s.args.ProduceTimeout)
	case s.buffer <- event:
	}
	return nil
}

func (s *memory) Subscribe(name string, bounds *broker.TriggerBounds, ccb backend.ConsumerDispatcher) error {
	if bounds != nil {
		return errors.New("bounds not supported for memory broker")
	}

	s.m.Lock()
	defer s.m.Unlock()
	s.ccbs[name] = ccb

	return nil
}

func (s *memory) Unsubscribe(name string) {
	s.m.Lock()
	defer s.m.Unlock()
	delete(s.ccbs, name)
}

func (s *memory) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.closing = true
	}()

	closing := false

	for {
		select {
		case event := <-s.buffer:
			s.fanOut(event)
		case <-ctx.Done():
			// signal to reject new events being produced
			s.closing = true
			close(s.buffer)

			// loop all remaining elements from the channel
			for event := range s.buffer {
				s.fanOut(event)
			}
			closing = true
		}

		if closing {
			break
		}
	}
	return nil
}

func (s *memory) fanOut(event *cloudevents.Event) {
	s.m.RLock()
	defer s.m.RUnlock()
	for _, ccb := range s.ccbs {
		ccb(event)
	}
}

func (s *memory) Probe(ctx context.Context) error {
	return nil
}
