// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/redis/go-redis/v9"
)

func (a *ReplayAdapter) ReplayEvents() error {
	ctx := cloudevents.ContextWithTarget(context.Background(), a.Sink)
	// Query the Redis database for everything in the "triggermesh" key
	val, err := a.Client.XRange(context.Background(), "triggermesh", "-", "+").Result()
	if err != nil {
		return fmt.Errorf("querying Redis database: %v", err)
	}
	// Get the events within the timestamps.
	events := a.getEventsWithinTimestamps(val, a.StartTime, a.EndTime)
	if err != nil {
		return fmt.Errorf("getting events within timestamps: %v", err)
	}
	// Filter the events if a filter is provided.
	a.Logger.Infof("filtering events with %s", a.Filter)
	events = a.filterEvents(events)
	// Send the events to the sink
	var eventCounter int
	startTime := time.Now()
	// if there are events to send, and a sink is provided, send the events.
	// otherwise,
	if len(events) != 0 && a.Sink != "" {
		for _, event := range events {
			a.Logger.Debugf("sending event #%s: %v", eventCounter, event)
			eventCounter++
			// create a new context with target set to the sink

			if result := a.CeClient.Send(ctx, event); !cloudevents.IsACK(result) {
				a.Logger.Errorf("Error sending event: %v", result.Error)
				return result
			}
		}
		endTime := time.Now()
		// Calculate the time it took to send the events.
		elapsedTime := endTime.Sub(startTime)
		// Calculate the average time it took to send an event.
		avgTime := elapsedTime / time.Duration(eventCounter)
		a.Logger.Infof("sent %d events in %v, average time per event: %v", eventCounter, elapsedTime, avgTime)
	} else {
		a.Logger.Infof("no events to send")
	}
	return nil
}

func (a *ReplayAdapter) filterEvents(events []cloudevents.Event) []cloudevents.Event {
	var filteredEvents []cloudevents.Event
	filter := a.Filter
	for _, event := range events {
		switch a.FilterKind {
		case "type":
			if event.Type() == filter {
				filteredEvents = append(filteredEvents, event)
			}
		case "source":
			if event.Source() == filter {
				filteredEvents = append(filteredEvents, event)
			}
		case "subject":
			if event.Subject() == filter {
				filteredEvents = append(filteredEvents, event)
			}
		case "datacontenttype":
			if event.DataContentType() == filter {
				filteredEvents = append(filteredEvents, event)
			}
		// case "extension":
		// 	if event.Extensions() == filter {
		// 		filteredEvents = append(filteredEvents, event)
		// 	}
		case "id":
			if event.ID() == filter {
				filteredEvents = append(filteredEvents, event)
			}
		case "schemaurl":
			// TODO: Fetch the provided schema URL and compare the data to the schema.
		}
	}
	return filteredEvents
}

func (a *ReplayAdapter) getEventsWithinTimestamps(val []redis.XMessage, start, end string) []cloudevents.Event {

	// if the start timestamp is empty set it to the start a long time ago
	if start == "" || start == "0" {
		start = "2020-02-13T16:01:12Z"
	}
	// if the end timestamp is empty or 0, set it to the current time
	if end == "" || end == "0" {
		// create a string of the current time in RFC3339 format
		end = fmt.Sprint(time.Now().Format(time.RFC3339))
	}
	// parse the start and end timestamps
	startTimestamp, err := time.Parse(time.RFC3339, start)
	if err != nil {
		a.Logger.Errorf("Error parsing start timestamp: %v", err)
		return nil
	}
	endTimestamp, err := time.Parse(time.RFC3339, end)
	if err != nil {
		a.Logger.Errorf("Error parsing start timestamp: %v", err)
		return nil
	}
	// create an array of events to return, if any are found.
	var events []cloudevents.Event
	// iterate through the messages received from the Redis stream.
	for _, msg := range val {
		b, err := json.Marshal(msg)
		if err != nil {
			a.Logger.Errorf("Error marshalling message: %v", err)
			continue
		}
		// unmarshal the message into an REvent struct
		var evnt REvent
		err = json.Unmarshal(b, &evnt)
		if err != nil {
			a.Logger.Errorf("Error unmarshalling message: %v", err)
			continue
		}
		// extract the '-0' from the event timestamp
		// and parse it to RFC3339
		parts := strings.Split(msg.ID, "-")
		unixTimestamp := parts[0]
		intunixtimestamp, err := strconv.Atoi(unixTimestamp)
		if err != nil {
			a.Logger.Errorf("Error parsing event timestamp: %v", err)
			continue
		}
		// parse the rfc3339 timestamp
		eventTimestamp, err := time.Parse(time.RFC3339, time.Unix(int64(intunixtimestamp)/1000, 0).Format(time.RFC3339))
		if err != nil {
			a.Logger.Errorf("Error parsing event timestamp: %v", err)
			continue
		}
		// if the event timestamp is between the start and end timestamps
		// then unmarshal it and add it to the events array.
		if eventTimestamp.After(startTimestamp) && eventTimestamp.Before(endTimestamp) {
			re := cloudevents.NewEvent()
			jsonStr := []byte(msg.Values["ce"].(string))
			err = json.Unmarshal(jsonStr, &re)
			if err != nil {
				a.Logger.Errorf("Error unmarshalling event: %v", err)
				continue
			}
			events = append(events, re)
		}
	}
	return events
}
