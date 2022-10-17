// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package lib

import (
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
)

const (
	tType   = "test.type"
	tSource = "test.source"
)

type CloudEventOption func(*cloudevents.Event)

func NewCloudEvent(opts ...CloudEventOption) cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetSource(tSource)
	event.SetType(tType)

	for _, opt := range opts {
		opt(&event)
	}

	return event
}

func CloudEventWithIDOption(id string) CloudEventOption {
	return func(e *cloudevents.Event) {
		e.SetID(id)
	}
}

func CloudEventWithTypeOption(t string) CloudEventOption {
	return func(e *cloudevents.Event) {
		e.SetType(t)
	}
}

func CloudEventWithSourceOption(s string) CloudEventOption {
	return func(e *cloudevents.Event) {
		e.SetSource(s)
	}
}

func CloudEventWithExtensionOption(key, value string) CloudEventOption {
	return func(e *cloudevents.Event) {
		e.Context.SetExtension(key, value)
	}
}
