// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package status

import "time"

type Status struct {
	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	LastUpdated   *time.Time                    `json:"lastUpdated,omitempty"`
	Ingest        IngestStatus                  `json:"ingest,omitempty"`
	Subscriptions map[string]SubscriptionStatus `json:"subscriptions,omitempty"`
}

type IngestStatus struct {
	Status  string  `json:"status"`
	Message *string `json:"message,omitempty"`

	// LastIngested event into the broker.
	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	LastIngested *string `json:"lastIngested,omitempty"`
}

type SubscriptionStatus struct {
	Status  string
	Message *string

	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	LastSent *time.Time `json:"lastSent,omitempty"`

	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	LastFiltered *time.Time `json:"lastFiltered,omitempty"`
}
