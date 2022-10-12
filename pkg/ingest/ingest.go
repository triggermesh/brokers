// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/triggermesh/brokers/pkg/config"
)

type CloudEventHandler func(context.Context, *cloudevents.Event) error
type ProbeHandler func() error

type Instance struct {
	port int

	ceHandler    CloudEventHandler
	probeHandler ProbeHandler

	logger *zap.SugaredLogger
}

type InstanceOption func(*Instance)

func NewInstance(logger *zap.SugaredLogger, opts ...InstanceOption) *Instance {
	i := &Instance{
		port:   8080,
		logger: logger,
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

func (i *Instance) Start(ctx context.Context) error {
	if i.logger == nil {
		panic("logger is nil!")
	}

	r := mux.NewRouter()

	p, err := cloudevents.NewHTTP(
		cloudevents.WithPort(i.port),
	)
	if err != nil {
		return fmt.Errorf("could not create a CloudEvents HTTP client: %w", err)
	}

	h, err := cloudevents.NewHTTPReceiveHandler(ctx, p, i.cloudEventsHandler)
	if err != nil {
		return fmt.Errorf("failed to create CloudEvents handler: %w", err)
	}

	r.Handle("/", h)

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := i.probeHandler(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, werr := w.Write([]byte(`{"ok":"false", "error":"` + err.Error() + `"}`))
			i.logger.Errorw("Could not write HTTP response (not healthy)", zap.Errors("error", []error{
				werr, err}))
			return
		}

		_, err = w.Write([]byte(`{"ok": "true"}`))
		i.logger.Errorw("Could not write HTTP response (healthy)", zap.Error(err))
	})

	address := fmt.Sprintf(":%d", i.port)
	srv := &http.Server{
		Addr:         address,
		WriteTimeout: time.Second * 10,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // Pass our instance of gorilla/mux in.
	}

	var srverr error
	go func() {
		i.logger.Info("Listening on " + address)
		if err := srv.ListenAndServe(); err != nil {
			srverr = fmt.Errorf("unable to start HTTP server, %s", err)
		}
	}()

	<-ctx.Done()

	if srverr != nil {
		return srverr
	}

	i.logger.Info("Exiting HTTP server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func (i *Instance) UpdateFromConfig(c *config.Config) {
	i.logger.Info("Ingest Server UpdateFromConfig ...")
}

func (i *Instance) RegisterCloudEventHandler(h CloudEventHandler) {
	i.ceHandler = h
}

func (i *Instance) RegisterProbeHandler(h ProbeHandler) {
	i.probeHandler = h
}

func (i *Instance) cloudEventsHandler(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, protocol.Result) {
	i.logger.Debug(fmt.Sprintf("Received CloudEvent: %v", event.String()))

	if i.ceHandler == nil {
		i.logger.Errorw("CloudEvent lost due to no handler configured")
		return nil, protocol.ResultNACK
	}

	if err := i.ceHandler(ctx, &event); err != nil {
		i.logger.Errorw("Could not produce CloudEvent to broker", zap.Error(err))
		return nil, protocol.ResultNACK
	}

	return nil, protocol.ResultACK
}
