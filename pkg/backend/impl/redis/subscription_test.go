// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareStreamIDs(t *testing.T) {
	testCases := map[string]struct {
		currentID     string
		endID         string
		expectedBool  bool
		expectedError *string
	}{
		"not exceeded by timestamp": {
			currentID:    "1586657816106-0",
			endID:        "1686657816106-0",
			expectedBool: false,
		},
		"not exceeded by id in same timestamp": {
			currentID:    "1586657816106-0",
			endID:        "1586657816106-1",
			expectedBool: false,
		},
		"exceeded by equal timestamp": {
			currentID:    "1586657816106-0",
			endID:        "1586657816106-0",
			expectedBool: true,
		},
		"exceeded by timestamp": {
			currentID:    "1686657816106-0",
			endID:        "1586657816106-0",
			expectedBool: true,
		},
		"equal timestamp, exceeded by id": {
			currentID:    "1686657816106-1",
			endID:        "1686657816106-0",
			expectedBool: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			eb := newExceedBounds(tc.endID)
			b := eb(tc.currentID)

			assert.Equal(t, tc.expectedBool, b)
		})
	}
}
