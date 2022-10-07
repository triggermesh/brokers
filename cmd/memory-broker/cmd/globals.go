// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"

	"go.uber.org/zap"
)

type Globals struct {
	context context.Context
	logger  *zap.SugaredLogger
}

func (g *Globals) SetLogger(logger *zap.SugaredLogger) {
	g.logger = logger
}

func (g *Globals) SetContext(ctx context.Context) {
	g.context = ctx
}
