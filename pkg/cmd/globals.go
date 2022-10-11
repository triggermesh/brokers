// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"

	"go.uber.org/zap"
)

type Globals struct {
	Context context.Context    `kong:"-"`
	Logger  *zap.SugaredLogger `kong:"-"`
}
