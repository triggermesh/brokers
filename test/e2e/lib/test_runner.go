// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package lib

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

// type Action func(t *testing.T)

// type ScheduleItem struct {
// 	Name   string
// 	On     Event
// 	Do     Action
// 	Status Status
// }

// type Schedule struct {
// 	Items map[Event]ScheduleItem
// }

// func (s *Schedule) validate() error {
// 	// Ensure there is an init
// 	if _, ok := s.Items[EventInit]; !ok {
// 		return errors.New("schedule needs an " + EventInit + "entry")
// 	}

// 	// Ensure that no schedule elements are unlinked

// 	return nil
// }

// func (s *Schedule) Run(t *testing.T) {
// 	err := s.validate()
// 	require.NoError(t, err, "Schedule for test is not valid.")
// }

type RunnerComponentStatus string

const (
	RunnerComponentStatusStopped  RunnerComponentStatus = "stopped"
	RunnerComponentStatusStarted  RunnerComponentStatus = "started"
	RunnerComponentStatusStopping RunnerComponentStatus = "stopping"
)

type ManagedBroker struct {
	Instance *broker.Instance
	Globals  *pkgcmd.Globals
	Cancel   context.CancelFunc
	Status   RunnerComponentStatus
}

type ManagedConsumer struct {
	Consumer Consumer
	Cancel   context.CancelFunc
	Status   RunnerComponentStatus
}

type BrokerTestRunner struct {
	// globals   *pkgcmd.Globals

	brokers   map[string]*ManagedBroker
	producers map[string]Producer
	consumers map[string]*ManagedConsumer

	t       *testing.T
	mainCtx context.Context
	zapcore zapcore.Core
	logs    *observer.ObservedLogs
	logger  *zap.Logger
}

// **********************************
// Methods for core Runner management
// **********************************

func NewBrokerTestRunner(ctx context.Context, t *testing.T) *BrokerTestRunner {
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)

	return &BrokerTestRunner{
		brokers:   make(map[string]*ManagedBroker),
		producers: make(map[string]Producer),
		consumers: make(map[string]*ManagedConsumer),

		mainCtx: ctx,
		zapcore: observedZapCore,
		t:       t,
		logs:    observedLogs,
		logger:  zap.New(observedZapCore),
	}
}

func (r *BrokerTestRunner) GetLogger() *zap.Logger {
	return r.logger
}

func (r *BrokerTestRunner) CleanUp() {
	for n, b := range r.brokers {
		err := os.Remove(b.Globals.ConfigPath)
		require.NoErrorf(r.t, err, "Failed to remove temporary configuration file for broker %s", n)
	}
}

// *****************************
// Methods for broker management
// *****************************

func (r *BrokerTestRunner) AddBroker(name string, port int, backend backend.Interface) {
	cfgfile, err := os.CreateTemp("", fmt.Sprintf("broker-%s-*.conf", name))
	require.NoError(r.t, err, "Failed to create configuration for broker")
	cfgfile.Close()

	observedLogger := zap.New(r.zapcore)

	g := &pkgcmd.Globals{
		Logger:     observedLogger.Sugar(),
		Port:       port,
		ConfigPath: cfgfile.Name(),
	}

	i, err := broker.NewInstance(g, backend)
	require.NoError(r.t, err, "Could not create backend.")

	r.brokers[name] = &ManagedBroker{
		Globals:  g,
		Instance: i,
		Status:   RunnerComponentStatusStopped,
	}
}

func (r *BrokerTestRunner) GetBroker(name string) *ManagedBroker {
	b, ok := r.brokers[name]
	require.Truef(r.t, ok, "Broker %s does not exists", name)

	return b
}

func (r *BrokerTestRunner) GetBrokerGlobals(name string) *pkgcmd.Globals {
	b, ok := r.brokers[name]
	require.Truef(r.t, ok, "Broker %s does not exists", name)

	return b.Globals
}

func (r *BrokerTestRunner) GetBrokerEndPoint(name string) string {
	b, ok := r.brokers[name]
	require.Truef(r.t, ok, "Broker %s does not exists", name)

	return fmt.Sprintf("http://0.0.0.0:%d", b.Globals.Port)
}

func (r *BrokerTestRunner) StartBroker(name string) {
	b, ok := r.brokers[name]
	require.Truef(r.t, ok, "Broker %s does not exists", name)

	// We use the context cancel for
	ctx, cancel := context.WithCancel(r.mainCtx)
	b.Cancel = cancel

	go func() {
		err := b.Instance.Start(ctx)
		require.NoError(r.t, err, "Error running broker instance")
		<-ctx.Done()
		b.Status = RunnerComponentStatusStopped
	}()

	b.Status = RunnerComponentStatusStarted
}

func (r *BrokerTestRunner) StopBroker(name string) {
	b, ok := r.brokers[name]
	require.Truef(r.t, ok, "Broker %s does not exists", name)

	b.Status = RunnerComponentStatusStopping
	b.Cancel()
}

func (r *BrokerTestRunner) UpdateBrokerConfig(name string, cfg *config.Config) {
	b, ok := r.brokers[name]
	require.Truef(r.t, ok, "Broker %s does not exists", name)

	cfgb, err := yaml.Marshal(cfg)
	require.NoError(r.t, err, "Failed to create configuration for broker")

	f, err := os.OpenFile(b.Globals.ConfigPath, os.O_RDWR, 0)
	require.NoError(r.t, err, "Failed to open configuration file for broker")

	defer func() {
		err := f.Close()
		assert.NoError(r.t, err, "Coudl not close temporary config file")
	}()

	err = f.Truncate(0)
	require.NoError(r.t, err, "Failed to erase previous configuration contents for broker")

	_, err = f.WriteAt(cfgb, 0)
	require.NoError(r.t, err, "Failed to write configuration file for broker")
}

type LogFilterOption func(le observer.LoggedEntry) bool

func LogFilterWithLevel(level zapcore.Level) LogFilterOption {
	return func(le observer.LoggedEntry) bool {
		return le.Level == level
	}
}

func LogFilterWithMessage(msg string) LogFilterOption {
	return func(le observer.LoggedEntry) bool {
		return le.Message == msg
	}
}

func LogFilterWithField(field zapcore.Field) LogFilterOption {
	return func(le observer.LoggedEntry) bool {
		for _, ctxField := range le.Context {
			if ctxField.Equals(field) {
				return true
			}
		}
		return false
	}
}

func FilterLogs(opts ...LogFilterOption) func(observer.LoggedEntry) bool {
	return func(le observer.LoggedEntry) bool {
		for _, opt := range opts {
			if !opt(le) {
				return false
			}
		}

		return true
	}
}

func (r *BrokerTestRunner) WaitForLogEntry(timeout time.Duration, opts ...LogFilterOption) bool {
	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logs := r.logs.Filter(FilterLogs(opts...))
			if logs.Len() != 0 {
				return true
			}

		case <-timeoutCh:
			return false
		}
	}
}

func (r *BrokerTestRunner) GetBrokerStatus(name string) RunnerComponentStatus {
	b, ok := r.brokers[name]
	require.Truef(r.t, ok, "Broker %s does not exists", name)

	return b.Status
}

// *****************************
// Methods for consumer management
// *****************************

func (r *BrokerTestRunner) AddConsumer(name string, consumer Consumer) {
	r.consumers[name] = &ManagedConsumer{
		Consumer: consumer,
	}
}

func (r *BrokerTestRunner) StartConsumer(name string) {
	c, ok := r.consumers[name]
	require.Truef(r.t, ok, "Consumer %s does not exists", name)

	// We use the context cancel for
	ctx, cancel := context.WithCancel(r.mainCtx)
	c.Cancel = cancel

	go func() {
		err := c.Consumer.Start(ctx)
		require.NoError(r.t, err, "Error running consumer instance")
		<-ctx.Done()
		c.Status = RunnerComponentStatusStopped
	}()

	c.Status = RunnerComponentStatusStarted
}

func (r *BrokerTestRunner) StopConsumer(name string) {
	c, ok := r.consumers[name]
	require.Truef(r.t, ok, "Consumer %s does not exists", name)

	c.Status = RunnerComponentStatusStopping
	c.Cancel()
}

// *****************************
// Methods for other components
// *****************************

func (r *BrokerTestRunner) AddProducer(name string, producer Producer) {
	r.producers[name] = producer
}

func (r *BrokerTestRunner) GetObservedLogs() *observer.ObservedLogs {
	return r.logs
}
