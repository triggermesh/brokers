// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/redis/go-redis/v9"

	replay "github.com/triggermesh/brokers/pkg/replay/redis"
)

func main() {
	st := time.Now()
	fmt.Println("Starting replay adapter at", st)
	// verify that all required environment variables are set
	redisAddress := os.Getenv("REDIS_ADDRESS")
	if redisAddress == "" {
		log.Panic("REDIS_ADDRESS environment variable is not set")
	}
	sink := os.Getenv("K_SINK")
	if sink == "" {
		log.Panic("K_SINK environment variable is not set")
	}
	// optional environment variables
	// redisPassword := os.Getenv("REDIS_PASSWORD")
	filter := os.Getenv("FILTER")
	filterKind := os.Getenv("FILTER_KIND")
	startTime := os.Getenv("START_TIME")
	// if a start time is provided, verify that it contains a valid timestamp
	if startTime != "" {
		_, err := strconv.ParseInt(startTime, 10, 64)
		if err != nil {
			log.Panicf("START_TIME environment variable is not a valid timestamp: %v", err)
		}
	}
	endTime := os.Getenv("END_TIME")
	// if an end time is provided, verify that it contains a valid timestamp
	if endTime != "" {
		_, err := strconv.ParseInt(endTime, 10, 64)
		if err != nil {
			log.Panicf("END_TIME environment variable is not a valid timestamp: %v", err)
		}
	}
	// Create a new Redis client
	client := redis.NewClient(&redis.Options{
		Addr: redisAddress,
		DB:   0,
	})
	// vierify that we can connect to the redis server
	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		log.Panicf("Error connecting to Redis server: %v", err)
	}
	// create a new cloudevent client
	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}
	// create a new zap logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal(err)
		return
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
	if err := replayAdapter.ReplayEvents(); err != nil {
		log.Fatalf("failed to start replayAdapter, %v", err)
	}
}
