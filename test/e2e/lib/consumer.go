//go:build e2e
// +build e2e

package lib

import (
	"context"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type StoredEvent struct {
	Time  time.Time
	Event cloudevents.Event
}

type Store struct {
	elements []StoredEvent
	mutex    sync.RWMutex
}

func (s *Store) Add(event cloudevents.Event) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.elements = append(s.elements, StoredEvent{
		Time:  time.Now(),
		Event: event,
	})
}

func (s *Store) GetAll() []StoredEvent {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var res []StoredEvent
	res = append(res, s.elements...)
	return res
}

type Consumer struct {
	client cloudevents.Client
	Store  Store
}

func NewConsumer(port int) (*Consumer, error) {
	client, err := cloudevents.NewClientHTTP(
		cloudevents.WithPort(port),
	)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		client: client,
	}, nil
}

func (s *Consumer) Start(ctx context.Context) error {
	return s.client.StartReceiver(ctx, s.receive)
}

func (s *Consumer) receive(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, cloudevents.Result) {
	s.Store.Add(event)
	return nil, cloudevents.ResultACK
}
