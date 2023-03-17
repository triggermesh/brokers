// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ReplayAdapter is the main struct for the replay adapter.
// it conains all the necessary information needed to replay events.
type ReplayAdapter struct {
	Sink       string
	CeClient   cloudevents.Client
	Logger     *zap.SugaredLogger
	Client     *redis.Client
	StartTime  time.Time
	EndTime    time.Time
	Filter     string
	FilterKind string
}

// REvent represents the structure of an expected Cloudevent stored
// in Redis via the XMessage interface.
type REvent struct {
	ID string `json:"ID"`
	Ce []struct {
		Specversion     string `json:"specversion"`
		ID              string `json:"id"`
		Source          string `json:"source"`
		Type            string `json:"type"`
		Datacontenttype string `json:"datacontenttype"`
		Data            string `json:"data"`
	} `json:"ce"`
}
