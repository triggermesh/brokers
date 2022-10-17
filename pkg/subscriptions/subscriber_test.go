// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package subscriptions

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/client/test"
	"github.com/stretchr/testify/assert"
	"github.com/triggermesh/brokers/pkg/backend/impl/memory"
	cfgbroker "github.com/triggermesh/brokers/pkg/config/broker"
	"github.com/triggermesh/brokers/test/lib"
)

var (
	// Each element must have an unique ID
	eventPool = []cloudevents.Event{
		lib.NewCloudEvent(
			lib.CloudEventWithIDOption("t1s1"),
			lib.CloudEventWithTypeOption("type1"),
			lib.CloudEventWithSourceOption("source1")),
		lib.NewCloudEvent(
			lib.CloudEventWithIDOption("t1s1ex2"),
			lib.CloudEventWithTypeOption("type1"),
			lib.CloudEventWithSourceOption("source1"),
			lib.CloudEventWithExtensionOption("ext2", "val2")),
		lib.NewCloudEvent(
			lib.CloudEventWithIDOption("t2s1ex1"),
			lib.CloudEventWithTypeOption("type2"),
			lib.CloudEventWithSourceOption("source1"),
			lib.CloudEventWithExtensionOption("ext1", "val1")),
		lib.NewCloudEvent(
			lib.CloudEventWithIDOption("t2s2ex1"),
			lib.CloudEventWithTypeOption("type2"),
			lib.CloudEventWithSourceOption("source2"),
			lib.CloudEventWithExtensionOption("ext1", "val1")),
		lib.NewCloudEvent(
			lib.CloudEventWithIDOption("t2s2ex2"),
			lib.CloudEventWithTypeOption("type2"),
			lib.CloudEventWithSourceOption("source2"),
			lib.CloudEventWithExtensionOption("ext2", "val2")),
	}
)

func TestSubscriberFilter(t *testing.T) {
	testCases := map[string]struct {
		trigger     cfgbroker.Trigger
		events      []cloudevents.Event
		expectedIds []string
	}{
		"no filter": {
			trigger:     cfgbroker.Trigger{},
			events:      eventPool,
			expectedIds: []string{"t1s1", "t1s1ex2", "t2s1ex1", "t2s2ex1", "t2s2ex2"},
		},

		"exact type": {
			trigger: cfgbroker.Trigger{
				Filters: []cfgbroker.Filter{
					{
						Exact: map[string]string{
							"type": "type1",
						},
					},
				},
			},
			events:      eventPool,
			expectedIds: []string{"t1s1", "t1s1ex2"},
		},

		"exact extension": {
			trigger: cfgbroker.Trigger{
				Filters: []cfgbroker.Filter{
					{
						Prefix: map[string]string{
							"ext2": "val",
						},
					},
				},
			},
			events:      eventPool,
			expectedIds: []string{"t1s1ex2", "t2s2ex2"},
		},

		"suffix source": {
			trigger: cfgbroker.Trigger{
				Filters: []cfgbroker.Filter{
					{
						Suffix: map[string]string{
							"source": "2",
						},
					},
				},
			},
			events:      eventPool,
			expectedIds: []string{"t2s2ex1", "t2s2ex2"},
		},

		"all and not": {
			trigger: cfgbroker.Trigger{
				Filters: []cfgbroker.Filter{
					{
						All: []cfgbroker.Filter{
							{
								Exact: map[string]string{
									"source": "source1",
								},
							}, {
								Not: &cfgbroker.Filter{
									Exact: map[string]string{
										"type": "type1",
									},
								},
							},
						},
					},
				},
			},
			events:      eventPool,
			expectedIds: []string{"t2s1ex1"},
		},

		"any and not prefix": {
			trigger: cfgbroker.Trigger{
				Filters: []cfgbroker.Filter{
					{
						Any: []cfgbroker.Filter{
							{
								Exact: map[string]string{
									"source": "source1",
								},
							}, {
								Not: &cfgbroker.Filter{
									Prefix: map[string]string{
										"ext2": "va",
									},
								},
							},
						},
					},
				},
			},
			events:      eventPool,
			expectedIds: []string{"t1s1", "t1s1ex2", "t2s1ex1", "t2s2ex1"},
		},
	}

	logger := zaptest.NewLogger(t).Sugar()
	ctx := context.Background()

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			b := memory.New(&memory.MemoryArgs{
				BufferSize:     1000,
				ProduceTimeout: 10 * time.Second,
			}, logger)

			client, rcv := cetest.NewMockRequesterClient(t, len(tc.events), testReceiver)
			s := subscriber{
				backend:   b,
				name:      "test-subscriber",
				ceClient:  client,
				parentCtx: ctx,
				logger:    logger,
			}

			s.updateTrigger(tc.trigger)
			for _, ev := range tc.events {
				s.dispatchCloudEvent(&ev)
			}

			exitLoop := false
			rcvEvents := []cloudevents.Event{}
			for !exitLoop {
				select {
				case e := <-rcv:
					rcvEvents = append(rcvEvents, e)
				case <-time.After(100 * time.Millisecond):
					exitLoop = true
				}
			}

			assert.Equal(t, len(tc.expectedIds), len(rcvEvents), "Unexpected number of received elements")
			for _, k := range tc.expectedIds {
				found := false
				for _, e := range rcvEvents {
					if e.ID() == k {
						found = true
						break
					}
				}

				if !found {
					assert.Failf(t, "Expected event did not pass the filter", "Event ID %s not received", k)
				}
			}
		})
	}
}

func testReceiver(inMessage cloudevents.Event) (*cloudevents.Event, cloudevents.Result) {
	return nil, cloudevents.ResultACK
}
