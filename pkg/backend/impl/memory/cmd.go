// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"fmt"
	"strings"
	"time"

	"github.com/rickb777/date/period"
)

type MemoryArgs struct {
	BufferSize     int    `help:"Number of events that can be hosted in the backend." env:"BUFFER_SIZE" default:"10000"`
	ProduceTimeout string `help:"Maximum wait time for producing an event to the backend." env:"PRODUCE_TIMEOUT" default:"PT5S"`

	ProduceTimeoutDuration time.Duration `kong:"-"`
}

func (ma *MemoryArgs) Validate() error {
	msg := []string{}

	if ma.ProduceTimeout != "" {
		p, err := period.Parse(ma.ProduceTimeout)
		if err != nil {
			msg = append(msg, fmt.Sprintf("Produce timeout is not an ISO8601 duration: %v", err))
		} else {
			ma.ProduceTimeoutDuration = p.DurationApprox()
		}
	}

	if len(msg) == 0 {
		return nil
	}

	return fmt.Errorf(strings.Join(msg, " "))
}
