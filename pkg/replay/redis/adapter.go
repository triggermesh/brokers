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
	pkgadapter "knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/logging"

	targetce "github.com/triggermesh/triggermesh/pkg/targets/adapter/cloudevents"
)

// NewAdapter adapter implementation
func NewAdapter(ctx context.Context, envAcc pkgadapter.EnvConfigAccessor, ceClient cloudevents.Client) pkgadapter.Adapter {
	env := envAcc.(*envAccessor)
	logger := logging.FromContext(ctx)

	// Create a new Redis client
	client := redis.NewClient(&redis.Options{
		Addr: env.RedisAddress,
		DB:   0,
	})
	_, err := client.Ping(ctx).Result()
	if err != nil {
		logger.Panicf("Error connecting to Redis server: %v", err)
	}

	replier, err := targetce.New(env.Component, logger.Named("replier"),
		targetce.ReplierWithStatefulHeaders(env.BridgeIdentifier),
		targetce.ReplierWithStaticResponseType("io.triggermesh.replay.response"),
		targetce.ReplierWithPayloadPolicy(targetce.PayloadPolicy(env.CloudEventPayloadPolicy)))
	if err != nil {
		logger.Panicf("Error creating CloudEvents replier: %v", err)
	}

	return &replayadapter{
		sink:       env.Sink,
		replier:    replier,
		ceClient:   ceClient,
		logger:     logger,
		client:     client,
		startTime:  env.StartTime,
		endTime:    env.EndTime,
		filter:     env.Filter,
		filterKind: env.FilterKind,
	}
}

var _ pkgadapter.Adapter = (*replayadapter)(nil)

func (a *replayadapter) Start(ctx context.Context) error {
	a.logger.Info("connecting to Redis")
	_, err := a.client.Ping(ctx).Result()
	if err != nil {
		a.logger.Panicf("Error connecting to Redis server: %v", err)
	}
	a.logger.Info("initating replay")
	if err := a.replayEvents(ctx); err != nil {
		a.logger.Panicf("Error replaying events: %v", err)
	}
	return nil
}

func (a *replayadapter) replayEvents(context.Context) error {
	// Query the Redis database for everything in the "triggermesh" key
	val, err := a.client.XRange(context.Background(), "triggermesh", a.startTime, a.endTime).Result()
	if err != nil {
		return fmt.Errorf("querying Redis database: %v", err)
	}
	// Get the events within the timestamps.
	events := a.getEventsWithinTimestamps(val, a.startTime, a.endTime)
	if err != nil {
		return fmt.Errorf("getting events within timestamps: %v", err)
	}
	// Filter the events if a filter is provided.
	if a.filter != "" {
		events = a.filterEvents(events)
	}
	// Send the events to the sink
	var eventCounter int
	startTime := time.Now()
	a.logger.Infof("sending %d events", len(events))
	for _, event := range events {
		a.logger.Debugf("sending event #%s: %v", eventCounter, event)
		eventCounter++
		if result := a.ceClient.Send(context.Background(), event); !cloudevents.IsACK(result) {
			a.logger.Errorf("Error sending event: %v", result.Error)
			return result
		}

	}
	endTime := time.Now()
	// Calculate the time it took to send the events.
	elapsedTime := endTime.Sub(startTime)
	// Calculate the average time it took to send an event.
	avgTime := elapsedTime / time.Duration(eventCounter)
	a.logger.Infof("sent %d events in %v, average time per event: %v", eventCounter, elapsedTime, avgTime)
	return nil
}

func (a *replayadapter) filterEvents(events []cloudevents.Event) []cloudevents.Event {
	var filteredEvents []cloudevents.Event
	filter := a.filter
	for _, event := range events {
		switch a.filterKind {
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

func (a *replayadapter) getEventsWithinTimestamps(val []redis.XMessage, start, end string) []cloudevents.Event {
	// create an array of events to return, if any are found.
	var events []cloudevents.Event
	// if the start and end timestamps are empty or 0, return all events
	if (start == "" || start == "0") && (end == "" || end == "0") {
		return events
	}
	// if the start timestamp is empty or 0, set it to 0-0
	if start == "" || start == "0" {
		start = "0-0"
	}
	// parse the start and end timestamps
	startTimestamp, err := time.Parse(time.RFC3339, start)
	if err != nil {
		fmt.Println("Error parsing start timestamp")
		fmt.Println(err)
		return events
	}
	endTimestamp, err := time.Parse(time.RFC3339, end)
	if err != nil {
		fmt.Println("Error parsing end timestamp")
		fmt.Println(err)
		return events
	}
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
