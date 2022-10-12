// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

package redis

import (
	"context"
	"testing"

	"github.com/triggermesh/brokers/pkg/backend/impl/redis"
	"github.com/triggermesh/brokers/pkg/config"
	"github.com/triggermesh/brokers/test/e2e/lib"
)

func TestRedisBroker(t *testing.T) {
	ctx := context.Background()

	runner := lib.NewBrokerTestRunner(ctx, t)
	defer runner.CleanUp()

	// TODO customize to use non local brokers, this
	// args assume a local Redis listening on default port
	args := &redis.RedisArgs{}
	backend := redis.New(args, runner.GetGlobals().Logger.Named("redis"))
	runner.SetupBroker(backend)

	producer := lib.NewSimpleProducer(runner.GetBrokerEndPoint())
	runner.AddProducer("producer", producer)

	consumer := lib.NewSimpleConsumer(9090)
	runner.AddConsumer("consumer", consumer)

	cfg := &config.Config{
		Triggers: map[string]config.Trigger{
			"test": {
				Target: config.Target{
					URL: consumer.GetConsumerEndPoint(),
				},
			},
		},
	}
	runner.UpdateBrokerConfig(cfg)

	// TODO run schedule

	// logs := runner.GetObservedLogs()
	// for _, l := range logs.All() {
	// 	t.Log(l.Message)

	// }
}
