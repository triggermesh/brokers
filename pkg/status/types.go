// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"time"
)

// Status of a broker instance.
type Status struct {
	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	LastUpdated   *time.Time                     `json:"lastUpdated,omitempty"`
	Ingest        IngestStatus                   `json:"ingest,omitempty"`
	Subscriptions map[string]*SubscriptionStatus `json:"subscriptions,omitempty"`
}

// EqualSoftStatus compares two status instances core event data, not taking into account
// timestamps at each structure level.
//
// This function is not thread safe, it is up to the caller to make sure structures are
// not concurrently modified.
func (s *Status) EqualSoftStatus(in *Status) bool {
	// If ingest not equal, return false.
	if !s.Ingest.EqualSoftStatus(&in.Ingest) {
		return false
	}

	// If subscriptions have been added or deleted, return false.
	//
	// The case where the number match but the contents are not equal
	// is covered in the next block below.
	if len(s.Subscriptions) != len(in.Subscriptions) {
		return false
	}

	// Iterate all subscriptions and return false on any inequality.
	for k := range s.Subscriptions {

		// If subscription not found at incoming status.
		ins, ok := in.Subscriptions[k]
		if !ok {
			return false
		}

		// If subscription found at incoming status, but soft equal
		// of their contents is not true.
		if !s.Subscriptions[k].equalSoftStatus(ins) {
			return false
		}
	}

	return true
}

// EqualSoftStatus compares verbatim two status instances.
//
// This function is not thread safe, it is up to the caller to make sure structures are
// not concurrently modified.
func (s *Status) EqualStatus(in *Status) bool {
	// If ingest not equal, return false.
	if !s.Ingest.EqualStatus(&in.Ingest) {
		return false
	}

	// If subscriptions have been added or deleted, return false.
	//
	// The case where the number match but the contents are not equal
	// is covered in the next block below.
	if len(s.Subscriptions) != len(in.Subscriptions) {
		return false
	}

	// Iterate all subscriptions and return false on any inequality.
	for k := range s.Subscriptions {

		// If subscription not found at incoming status.
		ins, ok := in.Subscriptions[k]
		if !ok {
			return false
		}

		// If subscription found at incoming status, but equal
		// of their contents is not true.
		if !s.Subscriptions[k].equalStatus(ins) {
			return false
		}
	}

	if s.LastUpdated == nil && in.LastUpdated != nil ||
		s.LastUpdated != nil && in.LastUpdated == nil ||
		(s.LastUpdated != nil && in.LastUpdated != nil && *s.LastUpdated != *in.LastUpdated) {
		return false
	}

	return true
}

type IngestStatus struct {
	Status  string  `json:"status"`
	Message *string `json:"message,omitempty"`

	// LastIngested event into the broker.
	LastIngested *time.Time `json:"lastIngested,omitempty"`
}

func (is *IngestStatus) EqualSoftStatus(in *IngestStatus) bool {
	if is.Message == nil && in.Message != nil ||
		is.Message != nil && in.Message == nil ||
		(is.Message != nil && in.Message != nil && *is.Message != *in.Message) {
		return false
	}

	return is.Status == in.Status
}

func (is *IngestStatus) EqualStatus(in *IngestStatus) bool {
	if !is.EqualSoftStatus(in) {
		return false
	}

	if is.LastIngested == nil && in.LastIngested != nil ||
		is.LastIngested != nil && in.LastIngested == nil ||
		(is.LastIngested != nil && in.LastIngested != nil && *is.LastIngested != *in.LastIngested) {
		return false
	}

	return true
}

type SubscriptionStatus struct {
	Status  string
	Message *string

	LastSent     *time.Time `json:"lastSent,omitempty"`
	LastFiltered *time.Time `json:"lastFiltered,omitempty"`
}

func (ss *SubscriptionStatus) equalSoftStatus(in *SubscriptionStatus) bool {
	if ss.Message == nil && in.Message != nil ||
		ss.Message != nil && in.Message == nil ||
		(ss.Message != nil && in.Message != nil && *ss.Message != *in.Message) {
		return false
	}

	return ss.Status == in.Status
}

func (ss *SubscriptionStatus) equalStatus(in *SubscriptionStatus) bool {
	if !ss.equalSoftStatus(in) {
		return false
	}

	if ss.LastSent == nil && in.LastSent != nil ||
		ss.LastSent != nil && in.LastSent == nil ||
		(ss.LastSent != nil && in.LastSent != nil && *ss.LastSent != *in.LastSent) {
		return false
	}

	if ss.LastFiltered == nil && in.LastFiltered != nil ||
		ss.LastFiltered != nil && in.LastFiltered == nil ||
		(ss.LastFiltered != nil && in.LastFiltered != nil && *ss.LastFiltered != *in.LastFiltered) {
		return false
	}
	return true
}
