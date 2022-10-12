// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package lib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/triggermesh/brokers/pkg/backend"
	"github.com/triggermesh/brokers/pkg/broker"
	pkgcmd "github.com/triggermesh/brokers/pkg/broker/cmd"
	"github.com/triggermesh/brokers/pkg/config"
)

type Status string
type Event string

const (
	StatusPending  Status = "pending"
	StatusStarted  Status = "started"
	StatusFinished Status = "finished"

	EventInit = "init"
)

type Action func(t *testing.T)

type ScheduleItem struct {
	Name   string
	On     Event
	Do     Action
	Status Status
}

type Schedule struct {
	Items map[Event]ScheduleItem
}

func (s *Schedule) validate() error {
	// Ensure there is an init
	if _, ok := s.Items[EventInit]; !ok {
		return errors.New("schedule needs an " + EventInit + "entry")
	}

	// Ensure that no schedule elements are unlinked

	return nil
}

func (s *Schedule) Run(t *testing.T) {
	err := s.validate()
	require.NoError(t, err, "Schedule for test is not valid.")
}

type BrokerTestRunner struct {
	globals   *pkgcmd.Globals
	broker    *broker.Instance
	producers map[string]Producer
	consumers map[string]Consumer
	schedule  Schedule

	t    *testing.T
	logs *observer.ObservedLogs
}

func NewBrokerTestRunner(ctx context.Context, t *testing.T) *BrokerTestRunner {
	cfgfile, err := os.CreateTemp("", "broker-*.conf")
	require.NoError(t, err, "Failed to create configuration for broker")
	cfgfile.Close()

	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	observedLogger := zap.New(observedZapCore)

	return &BrokerTestRunner{
		globals: &pkgcmd.Globals{
			Logger:     observedLogger.Sugar(),
			Port:       8080,
			Context:    ctx,
			ConfigPath: cfgfile.Name(),
		},

		producers: make(map[string]Producer),
		consumers: make(map[string]Consumer),

		t:    t,
		logs: observedLogs,
	}
}

func (r *BrokerTestRunner) GetGlobals() *pkgcmd.Globals {
	return r.globals
}

func (r *BrokerTestRunner) GetBrokerEndPoint() string {
	if r.broker == nil {
		return ""
	}

	return fmt.Sprintf("0.0.0.0:%d", r.globals.Port)
}

func (r *BrokerTestRunner) SetupBroker(backend backend.Interface) {
	b, err := broker.NewInstance(r.globals, backend)
	require.NoError(r.t, err, "Could not create backend.")

	r.broker = b
}

func (r *BrokerTestRunner) UpdateBrokerConfig(cfg *config.Config) {
	b, err := yaml.Marshal(cfg)
	require.NoError(r.t, err, "Failed to create configuration for broker")

	f, err := os.OpenFile(r.globals.ConfigPath, os.O_RDWR, 0)
	require.NoError(r.t, err, "Failed to open configuration file for broker")

	defer func() {
		err := f.Close()
		assert.NoError(r.t, err, "Coudl not close temporary config file")
	}()

	err = f.Truncate(0)
	require.NoError(r.t, err, "Failed to erase previous configuration contents for broker")

	_, err = f.WriteAt(b, 0)
	require.NoError(r.t, err, "Failed to write configuration file for broker")
}

func (r *BrokerTestRunner) SetupSchedule(schedule Schedule) {
	err := schedule.validate()
	require.NoError(r.t, err, "Could not setup schedule.")

	r.schedule = schedule
}

func (r *BrokerTestRunner) CleanUp() {
	if r.globals != nil {
		err := os.Remove(r.globals.ConfigPath)
		require.NoError(r.t, err, "Failed to remove temporary configuration file for broker")
	}
}

func (r *BrokerTestRunner) AddProducer(name string, producer Producer) {
	r.producers[name] = producer
}

func (r *BrokerTestRunner) AddConsumer(name string, consumer Consumer) {
	r.consumers[name] = consumer
}

func (r *BrokerTestRunner) GetObservedLogs() *observer.ObservedLogs {
	return r.logs
}
