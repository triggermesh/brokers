// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package config

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
- name: example1
- name: example2
`},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, err := Parse(tc.config)
			for _, trigger := range c.Triggers {
				t.Logf("%+v\n", trigger)
			}

			require.Equal(t, err, nil)
		})
	}
}
