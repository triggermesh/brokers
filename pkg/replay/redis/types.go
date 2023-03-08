// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package replay

import (
	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type ReplayAdapter struct {
	Sink       string
	CeClient   cloudevents.Client
	Logger     *zap.SugaredLogger
	Client     *redis.Client
	StartTime  string
	EndTime    string
	Filter     string
	FilterKind string
}

// REvent represents the structure of an expected Cloudevent stored
// in Reddis via the XMessage interface.
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
