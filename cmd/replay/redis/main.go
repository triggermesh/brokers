// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log"
	"os"
	"time"

	"go.uber.org/zap"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/redis/go-redis/v9"

	replay "github.com/triggermesh/brokers/pkg/replay/redis"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
	}
	st := time.Now()
	logger.Info("Starting replay adapter at", zap.Time("start_time", st))
	// verify that all required environment variables are set
	redisAddress := os.Getenv("REDIS_ADDRESS")
	if redisAddress == "" {
		logger.Panic("REDIS_ADDRESS environment variable is not set")
	}
	sink := os.Getenv("K_SINK")
	if sink == "" {
		logger.Panic("K_SINK environment variable is not set")
	}
	// optional environment variables
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisUser := os.Getenv("REDIS_USER")
	filter := os.Getenv("FILTER")
	filterKind := os.Getenv("FILTER_KIND")
	startTime := os.Getenv("START_TIME")
	endTime := os.Getenv("END_TIME")
	// Create a new Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddress,
		Password: redisPassword,
		Username: redisUser,
		DB:       0,
	})
	// verify that we can connect to the redis server
	_, err = client.Ping(context.Background()).Result()
	if err != nil {
		logger.Panic("failed to connect to redis server", zap.Error(err))
	}
	ctx := cloudevents.ContextWithTarget(context.Background(), a.Sink)
	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		logger.Panic("failed to create cloudevent client", zap.Error(err))
	}

	// create a new replayAdapter
	replayAdapter := &replay.ReplayAdapter{
		Sink:       sink,
		CeClient:   c,
		Client:     client,
		StartTime:  startTime,
		EndTime:    endTime,
		Filter:     filter,
		FilterKind: filterKind,
		Logger:     logger.Sugar(),
	}
	// start the replayAdapter
	if err := replayAdapter.ReplayEvents(ctx); err != nil {
		logger.Panic("failed to replay events", zap.Error(err))
	}
}
