// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package broker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	cases := map[string]struct {
		config string
	}{
		"simple": {
			config: `
triggers:
  trigger1:
  trigger2:
    fitlers:
    - exact:
        type: test.type
`},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, err := Parse(tc.config)
			require.Equal(t, err, nil)
			for _, trigger := range c.Triggers {
				t.Logf("%+v\n", trigger)
			}
		})
	}
}
