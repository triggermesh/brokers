// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/triggermesh/brokers/pkg/backend"
	"github.com/triggermesh/brokers/pkg/config"
)

type Instance struct {
	backend backend.Interface
	logger  *zap.Logger

	context         context.Context
	componentsRoute map[string]map[string]func(w http.ResponseWriter, r *http.Request)
}

func NewInstance(backend backend.Interface, logger *zap.Logger) *Instance {
	return &Instance{
		backend:         backend,
		logger:          logger,
		componentsRoute: map[string]map[string]func(w http.ResponseWriter, r *http.Request){},
	}
}

func (s *Instance) Start(ctx context.Context) error {
	r := mux.NewRouter()
	s.context = ctx

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		err := s.backend.Probe(r.Context())
		if err == nil {
			w.Write([]byte(`{"ok": "true"}`))
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"ok":"false", "error":"` + err.Error() + `"}`))
	})

	r.HandleFunc("/ingest/", s.cloudEventsHandler)
	// TODO Configure broker listener

	s.logger.Info("Listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		return fmt.Errorf("unable to start http server, %s", err)
	}

	return nil
}

func (s *Instance) UpdateFromConfig(c *config.Config) {
	s.logger.Info("Server UpdateFromConfig ...")
}

func (s *Instance) cloudEventsHandler(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("dealing with incoming CloudEvent")
}
