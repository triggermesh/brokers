// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	"github.com/triggermesh/brokers/pkg/backend"
)

func New(logger *zap.Logger) backend.Interface {
	return &memory{
		logger: logger,
	}
}

type memory struct {
	logger *zap.Logger
}

func (s *memory) Info() *backend.Info {
	return &backend.Info{
		Name: "Memory",
	}
}

func (s *memory) Init(ctx context.Context) error {
	return nil
}

func (s *memory) Disconnect() error {
	return nil
}

func (s *memory) Produce(ctx context.Context, event *cloudevents.Event) error {
	return nil
}

func (s *memory) Start(ctx context.Context, ccb backend.ConsumerDispatcher) {
}

func (s *memory) Probe(ctx context.Context) error {
	return nil
}
