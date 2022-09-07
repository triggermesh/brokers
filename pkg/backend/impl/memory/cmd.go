// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package memory

import "time"

type MemoryArgs struct {
	BufferSize     int           `help:"Number of events that can be hosted in the backend." default:"10000"`
	ProduceTimeout time.Duration `help:"Maximum wait time for producing an event to the backend." default:"5s"`
}
