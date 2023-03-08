// Copyright 2022 TriggerMesh Inc.
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
	// if a.Filter != "" {
	// 	a.Logger.Debugf("filtering events with %s", a.Filter)
	// 	events = a.filterEvents(events)
	// }
	// Send the events to the sink
	var eventCounter int
	startTime := time.Now()
	fmt.Printf("events %+v", events)
	fmt.Printf("sending %d events", len(events))
	// if there are events to send, and a sink is provided, send the events.
	// otherwise,
	if len(events) != 0 && a.Sink != "" {
		for _, event := range events {
			a.Logger.Debugf("sending event #%s: %v", eventCounter, event)
			eventCounter++
			// create a new context with target set to the sink
			ctx := cloudevents.ContextWithTarget(context.Background(), a.Sink)
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

	// if the start timestamp is empty set it to the start of time
	if start == "" || start == "0" {
		start = "2023-02-13T16:01:12Z"
	}
	// if the end timestamp is empty or 0, set it to the current time
	if end == "" || end == "0" {
		// create a timestamp of now in RCF3339 format
		now := time.Now()
		end = strconv.FormatInt(now.UnixNano(), 10)
		fmt.Printf("end %+v", end)
	}

	start = "2023-02-13T16:01:12Z"
	end = "2023-05-13T16:01:12Z"

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

	fmt.Printf("sorting events between %s and %s", startTimestamp, endTimestamp)
	fmt.Println("")
	fmt.Printf("val %+v", val)
	// create an array of events to return, if any are found.
	var events []cloudevents.Event
	// // if the start and end timestamps are empty or 0, return all events
	// // if start == "" || end == "" {
	// // iterate through the messages received from the Redis stream.
	// for _, msg := range val {
	// 	// marshal the message into a byte array
	// 	b, err := json.Marshal(msg)
	// 	if err != nil {
	// 		a.Logger.Errorf("Error marshalling msg: %v", err)
	// 		continue
	// 	}
	// 	// unmarshal the message into an REvent struct
	// 	var evnt REvent
	// 	err = json.Unmarshal(b, &evnt)
	// 	if err != nil {
	// 		a.Logger.Errorf("Error unmarshalling event: %v", err)
	// 		continue
	// 	}

	// 	for _, ce := range evnt.Ce {
	// 		// create a new CloudEvent
	// 		event := cloudevents.NewEvent()
	// 		// set the CloudEvent's type
	// 		event.SetType(ce.Type)
	// 		events = append(events, event)
	// 	}
	// }
	// return events
	// // }

	// iterate through the messages received from the Redis stream.
	for _, msg := range val {
		b, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("Error marshalling msg")
			fmt.Println(err)
			continue
		}
		// unmarshal the message into an REvent struct
		var evnt REvent
		err = json.Unmarshal(b, &evnt)
		if err != nil {
			fmt.Println("Error unmarshalling event")
			fmt.Println(err)
			continue
		}
		// extract the '-0' from the event timestamp
		// and parse it to RFC3339
		parts := strings.Split(msg.ID, "-")
		unixTimestamp := parts[0]
		intunixtimestamp, err := strconv.Atoi(unixTimestamp)
		if err != nil {
			fmt.Println("Error converting unix timestamp to int")
			fmt.Println(err)
			continue
		}
		// parse the rfc3339 timestamp
		eventTimestamp, err := time.Parse(time.RFC3339, time.Unix(int64(intunixtimestamp)/1000, 0).Format(time.RFC3339))
		if err != nil {
			fmt.Println("Error parsing event timestamp")
			fmt.Println(err)
			continue
		}
		// if the event timestamp is between the start and end timestamps
		// then unmarshal it and add it to the events array.
		if eventTimestamp.After(startTimestamp) && eventTimestamp.Before(endTimestamp) {
			re := cloudevents.NewEvent()
			jsonStr := []byte(msg.Values["ce"].(string))
			err = json.Unmarshal(jsonStr, &re)
			if err != nil {
				fmt.Println("Error unmarshalling event")
				fmt.Println(err)
				continue
			}
			events = append(events, re)
		}
	}
	return events
}
