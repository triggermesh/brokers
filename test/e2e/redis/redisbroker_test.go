// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

package redis

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/triggermesh/brokers/pkg/backend/impl/redis"
	cfgbroker "github.com/triggermesh/brokers/pkg/config/broker"
	"github.com/triggermesh/brokers/test/e2e"
	"github.com/triggermesh/brokers/test/lib"
)

var runner *e2e.BrokerTestRunner

func TestMain(m *testing.M) {
	InitializeRedisFlags()
	exit := m.Run()

	if exit != 0 {
		log.Printf("Error executing tests\n")
		ol := runner.GetObservedLogs()
		for _, l := range ol.All() {
			log.Printf("%s:%s: %s || %v\n", l.Level, l.LoggerName, l.Message, l.Context)
		}

		// TODO iterate producers and consumers, print their stored events
	}

	os.Exit(exit)
}

func TestRedisBroker(t *testing.T) {
	ctx := context.Background()

	runner = e2e.NewBrokerTestRunner(ctx, t)
	defer runner.CleanUp()

	// TODO customize to use non local brokers, this
	// args assume a local Redis listening on default port
	args := &redis.RedisArgs{
		Address:  Flags.RedisAddress,
		Password: Flags.RedisPassword,
		Stream:   Flags.RedisStream,
	}
	backend := redis.New(args, runner.GetLogger().Sugar().Named("redis"))
	runner.AddBroker("main", 18080, backend)

	producer := lib.NewSimpleProducer(runner.GetBrokerEndPoint("main"))
	runner.AddProducer("producer", producer)

	consumer := lib.NewSimpleConsumer(9090)
	runner.AddConsumer("consumer", consumer)

	cfg := &cfgbroker.Config{
		Triggers: map[string]cfgbroker.Trigger{
			"test1": {
				Target: cfgbroker.Target{
					URL: consumer.GetConsumerEndPoint(),
				},
			},
		},
	}

	runner.UpdateBrokerConfig("main", cfg)
	runner.StartConsumer("consumer")
	runner.StartBroker("main")

	// Make sure configuration was applied
	ok := runner.WaitForLogEntry(2*time.Second,
		e2e.LogFilterWithLevel(zap.InfoLevel),
		e2e.LogFilterWithMessage("Subscription for trigger updated"),
		e2e.LogFilterWithField(zap.String("name", "test1")))
	require.True(t, ok, "Timed out waiting for log condition on trigger configuration")

	// After configuration is applied, produce an event that should be
	// routed via configured triggers.
	ev := lib.NewCloudEvent()
	err := producer.Produce(ctx, ev)
	assert.NoError(t, err, "Error producing event")

	// wait for event to be seen at consumer
	ok = consumer.WaitForEvent(5*time.Second, lib.EventiFilterWithID(ev.ID()))
	require.True(t, ok, "Timed out waiting for event condition on consumer")

	runner.StopBroker("main")

	// TODO Some final debugging, remove this before merging
	ol := runner.GetObservedLogs()
	for _, l := range ol.All() {
		t.Logf("%s:%s: %s || %v", l.Level, l.LoggerName, l.Message, l.Context)
	}

	t.Logf("producer count %d", len(producer.GetStoredEvents()))
	for _, pe := range producer.GetStoredEvents() {
		t.Logf("produced event content: \n%v\n%v", pe.Event.String(), pe.Result)
	}

	t.Logf("consumer count %d", len(consumer.GetStoredEvents()))
	for _, pe := range consumer.GetStoredEvents() {
		t.Logf("consumed event content: \n%v\n%v", pe.Event.String(), pe.Result)
	}
}
