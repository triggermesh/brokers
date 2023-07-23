// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/triggermesh/brokers/pkg/config/broker"
)

var (
	tStartID = "1586657816106-1"
	tEndID   = "1686657816106-0"

	tStartDateMillis     = "2020-04-12T02:16:56.106+00:00"
	tStartDateMillisToId = "1586657816106"
	tEndDateMillis       = "2023-06-13T12:03:36.106+00:00"
	tEndDateMillisToId   = "1686657816106"

	tStartDateSeconds     = "2020-04-12T02:16:56Z"
	tStartDateSecondsToId = "1586657816000"
	tEndDateSeconds       = "2023-06-13T12:03:36+00:00"
	tEndDateSecondsToId   = "1686657816000"
)

func TestBoundsResolver(t *testing.T) {
	testCases := map[string]struct {
		bounds          *broker.TriggerBounds
		expectedStartID string
		expectedEndID   string
		expectedError   string
	}{
		"no bounds": {
			expectedStartID: "$",
		},
		"no bound contents": {
			bounds: &broker.TriggerBounds{},

			expectedStartID: "$",
		},
		"no bound by ID": {
			bounds: &broker.TriggerBounds{
				ByID: &broker.Bounds{},
			},
			expectedStartID: "$",
		},
		"bound by ID": {
			bounds: &broker.TriggerBounds{
				ByID: &broker.Bounds{
					Start: &tStartID,
					End:   &tEndID,
				},
			},
			expectedStartID: tStartID,
			expectedEndID:   tEndID,
		},
		"bound by Date, seconds": {
			bounds: &broker.TriggerBounds{
				ByDate: &broker.Bounds{
					Start: &tStartDateSeconds,
					End:   &tEndDateSeconds,
				},
			},
			expectedStartID: tStartDateSecondsToId,
			expectedEndID:   tEndDateSecondsToId,
		},
		"bound by Date, millis": {
			bounds: &broker.TriggerBounds{
				ByDate: &broker.Bounds{
					Start: &tStartDateMillis,
					End:   &tEndDateMillis,
				},
			},
			expectedStartID: tStartDateMillisToId,
			expectedEndID:   tEndDateMillisToId,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			start, end, err := boundsResolver(tc.bounds)
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}

			require.Nil(t, err)
			assert.Equal(t, tc.expectedStartID, start)
			assert.Equal(t, tc.expectedEndID, end)
		})
	}
}
