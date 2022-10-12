// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package lib

import (
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type StoredEventOption func(*StoredEvent)

type StoredEvent struct {
	Time   time.Time
	Event  *cloudevents.Event
	Result *cloudevents.Result
	Reply  *cloudevents.Event
}

func StoredEventWithResult(r cloudevents.Result) StoredEventOption {
	return func(se *StoredEvent) {
		se.Result = &r
	}
}

type Store struct {
	elements []StoredEvent
	mutex    sync.RWMutex
}

func (s *Store) Add(event cloudevents.Event, opts ...StoredEventOption) {
	se := StoredEvent{
		Time:  time.Now(),
		Event: &event,
	}

	for _, opt := range opts {
		opt(&se)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.elements = append(s.elements, se)
}

func (s *Store) GetAll() []StoredEvent {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var res []StoredEvent
	res = append(res, s.elements...)
	return res
}
