//go:build e2e
// +build e2e

package lib

import (
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	tType   = "test.type"
	tSource = "test.source"
)

func NewCloudEvent() cloudevents.Event {
	// Create an Event.
	event := cloudevents.NewEvent()
	event.SetSource(tSource)
	event.SetType(tType)

	return event
}
