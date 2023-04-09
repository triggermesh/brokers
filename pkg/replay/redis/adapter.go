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
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
	"knative.dev/pkg/logging"

	cfgbroker "github.com/triggermesh/brokers/pkg/config/broker"
)

func (a *ReplayAdapter) ReplayEvents(ctx context.Context) error {
	// Query the Redis database for everything in the "triggermesh" key
	val, err := a.Client.XRange(context.Background(), "default.demo", "-", "+").Result()
	if err != nil {
		return fmt.Errorf("querying Redis database: %v", err)
	}
	// Get the events within the timestamps.
	events := a.getEventsWithinTimestamps(val, a.StartTime, a.EndTime)
	if err != nil {
		return fmt.Errorf("getting events within timestamps: %w", err)
	}
	a.Logger.Infof("found %d events within timestamps", len(events))
	a.Logger.Infof("events: %+v", events)
	var eventCounter int
	startTime := time.Now()
	// create a new context with target set to the sink
	ctx = cloudevents.ContextWithTarget(context.Background(), a.Sink)
	// if there are events to send, and a sink is provided, send the events.
	// otherwise,
	if len(events) != 0 {
		for _, event := range events {
			materializedFilters := materializeFiltersList(context.Background(), a.Filter)
			res := subscriptionsapi.NewAllFilter(materializedFilters...).Filter(context.Background(), event)
			if res == eventfilter.FailFilter {
				a.Logger.Debugf("event #%s failed filter, skipping", eventCounter)
				continue
			}

			a.Logger.Debugf("sending event #%s: %v", eventCounter, event)

			if result := a.CeClient.Send(ctx, event); !cloudevents.IsACK(result) {
				a.Logger.Errorf("Error sending event: %v", result.Error)
			}
			eventCounter++
		}
		endTime := time.Now()
		// Calculate the time it took to send the events.
		elapsedTime := endTime.Sub(startTime)
		// Calculate the average time it took to send an event.
		if eventCounter > 0 {
			avgTime := elapsedTime / time.Duration(eventCounter)
			a.Logger.Infof("sent %d events in %v, average time per event: %v", eventCounter, elapsedTime, avgTime)
		} else {
			a.Logger.Infof("no events to send")
		}
	} else {
		a.Logger.Infof("no events to send")
	}
	return nil
}

func (a *ReplayAdapter) getEventsWithinTimestamps(val []redis.XMessage, start, end time.Time) []cloudevents.Event {
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
		// extract everything after '-' from the event timestamp
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
		if eventTimestamp.After(start) && eventTimestamp.Before(end) {
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

func materializeFiltersList(ctx context.Context, filters []cfgbroker.Filter) []eventfilter.Filter {
	materializedFilters := make([]eventfilter.Filter, 0, len(filters))
	for _, f := range filters {
		f := materializeSubscriptionsAPIFilter(ctx, f)
		if f == nil {
			logging.FromContext(ctx).Warnw("Failed to parse filter. Skipping filter.", zap.Any("filter", f))
			continue
		}
		materializedFilters = append(materializedFilters, f)
	}
	return materializedFilters
}

func materializeSubscriptionsAPIFilter(ctx context.Context, filter cfgbroker.Filter) eventfilter.Filter {
	var materializedFilter eventfilter.Filter
	var err error
	switch {
	case len(filter.Exact) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewExactFilter(filter.Exact)
		if err != nil {
			logging.FromContext(ctx).Debugw("Invalid exact expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.Prefix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewPrefixFilter(filter.Prefix)
		if err != nil {
			logging.FromContext(ctx).Debugw("Invalid prefix expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.Suffix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewSuffixFilter(filter.Suffix)
		if err != nil {
			logging.FromContext(ctx).Debugw("Invalid suffix expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.All) > 0:
		materializedFilter = subscriptionsapi.NewAllFilter(materializeFiltersList(ctx, filter.All)...)
	case len(filter.Any) > 0:
		materializedFilter = subscriptionsapi.NewAnyFilter(materializeFiltersList(ctx, filter.Any)...)
	case filter.Not != nil:
		materializedFilter = subscriptionsapi.NewNotFilter(materializeSubscriptionsAPIFilter(ctx, *filter.Not))
	}
	return materializedFilter
}
