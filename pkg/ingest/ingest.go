// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	obshttp "github.com/cloudevents/sdk-go/observability/opencensus/v2/http"
	ceclient "github.com/cloudevents/sdk-go/v2/client"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"go.uber.org/zap"

	cfgbroker "github.com/triggermesh/brokers/pkg/config/broker"
	"github.com/triggermesh/brokers/pkg/ingest/metrics"
	"github.com/triggermesh/brokers/pkg/status"
)

type CloudEventHandler func(context.Context, *cloudevents.Event) error
type ProbeHandler func() error

type Instance struct {
	port int

	ceHandler    CloudEventHandler
	probeHandler ProbeHandler

	statusManager status.Manager
	reporter      metrics.Reporter
	logger        *zap.SugaredLogger
}

type InstanceOption func(*Instance)

func NewInstance(reporter metrics.Reporter, logger *zap.SugaredLogger, opts ...InstanceOption) *Instance {
	i := &Instance{
		port:     8080,
		logger:   logger,
		reporter: reporter,
	}

	for _, opt := range opts {
		opt(i)
	}

	return i
}

func InstanceWithPort(port int) InstanceOption {
	return func(i *Instance) {
		i.port = port
	}
}

func InstanceWithStatusManager(sm status.Manager) InstanceOption {
	return func(i *Instance) {
		i.statusManager = sm
	}
}

func (i *Instance) Start(ctx context.Context) error {
	if i.logger == nil {
		panic("logger is nil!")
	}

	p, err := obshttp.NewObservedHTTP(
		cloudevents.WithPort(i.port),
		cloudevents.WithShutdownTimeout(10*time.Second),
		cloudevents.WithGetHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use common health paths.
			if r.URL.Path != "/healthz" && r.URL.Path != "/_ah/health" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			if err := i.probeHandler(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, werr := w.Write([]byte(`{"ok":"false", "error":"` + err.Error() + `"}`))
				i.logger.Errorw("Could not write HTTP response (not healthy)", zap.Errors("error", []error{
					werr, err}))
				return
			}

			if _, err := w.Write([]byte(`{"ok": "true"}`)); err != nil {
				i.logger.Errorw("Could not write HTTP response (healthy)", zap.Error(err))
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("could not create a CloudEvents HTTP client protocol: %w", err)
	}

	c, err := ceclient.New(p, ceclient.WithObservabilityService(
		metrics.NewOpenCensusObservabilityService(i.reporter)))
	if err != nil {
		return fmt.Errorf("failed to create CloudEvents client: %w", err)
	}

	i.logger.Infof("Listening on %d", i.port)
	var handler interface{}

	if i.statusManager != nil {
		// Notify and defer status manager
		i.statusManager.UpdateIngestStatus(&status.IngestStatus{
			Status: status.IngestStatusReady,
		})
		defer i.statusManager.UpdateIngestStatus(&status.IngestStatus{
			Status: status.IngestStatusClosed,
		})

		handler = i.cloudEventsStatusManagerHandler
	} else {
		handler = i.cloudEventsHandler
	}

	if err := c.StartReceiver(ctx, handler); err != nil {
		return fmt.Errorf("unable to start HTTP server: %w", err)
	}

	return nil
}

func (i *Instance) UpdateFromConfig(c *cfgbroker.Config) {
	i.logger.Info("Ingest Server UpdateFromConfig ...")
}

func (i *Instance) RegisterCloudEventHandler(h CloudEventHandler) {
	i.ceHandler = h
}

func (i *Instance) RegisterProbeHandler(h ProbeHandler) {
	i.probeHandler = h
}

func (i *Instance) cloudEventsStatusManagerHandler(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, protocol.Result) {
	e, p := i.cloudEventsHandler(ctx, event)

	t := time.Now()
	if i.statusManager != nil {
		i.statusManager.UpdateIngestStatus(&status.IngestStatus{
			Status:       status.IngestStatusRunning,
			LastIngested: &t,
		})
	}

	return e, p
}

func (i *Instance) cloudEventsHandler(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, protocol.Result) {
	i.logger.Debug(fmt.Sprintf("Received CloudEvent: %v", event.String()))

	if i.ceHandler == nil {
		i.logger.Errorw("CloudEvent lost due to no ingest handler configured")
		return nil, protocol.ResultNACK
	}

	if err := i.ceHandler(ctx, &event); err != nil {
		i.logger.Errorw("Could not produce CloudEvent to broker", zap.Error(err))
		return nil, protocol.ResultNACK
	}

	return nil, protocol.ResultACK
}
