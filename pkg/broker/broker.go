// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package broker

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/triggermesh/brokers/pkg/backend"
	"github.com/triggermesh/brokers/pkg/ingest"
	"github.com/triggermesh/brokers/pkg/subscriptions"
	"go.uber.org/zap"
)

type Instance struct {
	backend      backend.Interface
	ingest       *ingest.Instance
	subscription *subscriptions.Manager

	logger *zap.Logger
}

func NewInstance(backend backend.Interface, ingest *ingest.Instance, subscription *subscriptions.Manager, logger *zap.Logger) *Instance {
	return &Instance{
		backend:      backend,
		ingest:       ingest,
		logger:       logger,
		subscription: subscription,
	}
}

func (i *Instance) Start(inctx context.Context) error {
	sigctx, stop := signal.NotifyContext(inctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	grp, ctx := errgroup.WithContext(sigctx)

	// Initialization will create structures, execute migrations
	// and claim non processed messages from the backend.
	err := i.backend.Init(ctx)
	if err != nil {
		return fmt.Errorf("could not initialize backend: %v", err)
	}

	// Start is a blocking function that will read messages from the backend
	// implementation and send them to the generic dispatcher.
	// When the dispatcher returns the message is marked as processed.
	grp.Go(func() error {
		i.backend.Start(ctx, i.dispatch)
		return nil
	})

	// Disconnect from backend after subscription manager and
	// ingest server are done.
	defer i.backend.Disconnect()

	// Start the server that ingests CloudEvents and push sends
	// to
	grp.Go(func() error {
		err := i.ingest.Start(ctx)
		return err
	})

	return grp.Wait()
}

func (i *Instance) dispatch(event *cloudevents.Event) {
	i.logger.Info(fmt.Sprintf("Processing CloudEvent: %v", event))
	// subscription management should ocurr here.
}
