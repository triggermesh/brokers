// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net/url"
)

type Ingest struct {
	User     string
	Password string
}

type Target struct {
	URL            url.URL
	DeliveryOption DeliveryOption
}

type DeliveryOption struct {
	Retries          int
	RetriesAlgorithm string
	DeadLetterTarget *Target
}

type Filter struct {
}

type Trigger struct {
	Name    string
	Filters []Filter
	Targets []Target
}

type Config struct {
	Ingest   *Ingest
	Triggers []Trigger
}
