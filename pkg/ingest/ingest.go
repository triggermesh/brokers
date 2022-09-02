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
	ceHandler    CloudEventHandler
	probeHandler ProbeHandler

	logger *zap.Logger
}

func NewInstance(logger *zap.Logger) *Instance {
	return &Instance{
		logger: logger,
	}
}

func (s *Instance) Start(ctx context.Context) error {
	if s.logger == nil {
		panic("logger is nil!")
	}

	r := mux.NewRouter()

	p, err := cloudevents.NewHTTP()
	if err != nil {
		return fmt.Errorf("could not create a CloudEvents HTTP client: %w", err)
	}

	h, err := cloudevents.NewHTTPReceiveHandler(ctx, p, s.cloudEventsHandler)
	if err != nil {
		return fmt.Errorf("failed to create CloudEvents handler: %v", err.Error())
	}

	r.Handle("/", h)

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := s.probeHandler(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"ok":"false", "error":"` + err.Error() + `"}`))
			return
		}

		w.Write([]byte(`{"ok": "true"}`))
	})

	srv := &http.Server{
		Addr:         ":8080",
		WriteTimeout: time.Second * 10,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // Pass our instance of gorilla/mux in.
	}

	var srverr error
	go func() {
		s.logger.Info("Listening on :8080")
		if err := srv.ListenAndServe(); err != nil {
			srverr = fmt.Errorf("unable to start HTTP server, %s", err)
		}
	}()

	<-ctx.Done()

	if srverr != nil {
		return srverr
	}

	s.logger.Info("Exiting HTTP server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	return nil
}

func (s *Instance) UpdateFromConfig(c *config.Config) {
	s.logger.Info("Ingest Server UpdateFromConfig ...")
}

func (s *Instance) RegisterCloudEventHandler(h CloudEventHandler) {
	s.ceHandler = h
}

func (s *Instance) RegisterProbeHandler(h ProbeHandler) {
	s.probeHandler = h
}

func (s *Instance) cloudEventsHandler(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, protocol.Result) {
	s.logger.Debug(fmt.Sprintf("Received CloudEvent: %v", event.String()))

	if s.ceHandler == nil {
		s.logger.Error("CloudEvent lost due to no handler configured")
		return nil, protocol.ResultNACK
	}

	if err := s.ceHandler(ctx, &event); err != nil {
		s.logger.Error("Could not produce CloudEvent to broker", zap.Error(err))
		return nil, protocol.ResultNACK
	}

	return nil, protocol.ResultACK
}
