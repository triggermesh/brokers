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

func TestSubscriber(t *testing.T) {
	testCases := map[string]struct {
		trigger               cfgbroker.Trigger
		event                 cloudevents.Event
		expectedDispatchCount int
	}{
		"simple": {
			trigger:               cfgbroker.Trigger{},
			event:                 lib.NewCloudEvent(),
			expectedDispatchCount: 1,
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

			client, rcv := cetest.NewMockRequesterClient(t, 1, testReceiver)
			s := subscriber{
				backend:   b,
				name:      "test-subscriber",
				ceClient:  client,
				parentCtx: ctx,
				logger:    logger,
			}

			s.updateTrigger(tc.trigger)
			s.dispatchCloudEvent(&tc.event)

			count := 0
			exitLoop := false
			for !exitLoop {
				select {
				case <-rcv:
					count++
				case <-time.After(100 * time.Millisecond):
					exitLoop = true
				}
			}

			assert.Equal(t, tc.expectedDispatchCount, count, "Unexpected number of received elements")
		})
	}
}

func testReceiver(inMessage cloudevents.Event) (*cloudevents.Event, cloudevents.Result) {
	return nil, cloudevents.ResultACK
}
