// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"net/url"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"

	"knative.dev/pkg/apis"
)

type Ingest struct {
	User     string
	Password string
}

func (i *Ingest) Validate(ctx context.Context) *apis.FieldError {
	if i == nil {
		return nil
	}

	if i.Password != "" && i.User == "" {
		return &apis.FieldError{
			Message: "user must be provided when password is informed",
			Paths:   []string{"user"},
		}
	}

	return nil
}

type DeliveryOption struct {
	Retries          int
	RetriesAlgorithm string
	DeadLetterTarget *Target
}

type Target struct {
	URL            string
	DeliveryOption DeliveryOption
}

func (i *Target) Validate(ctx context.Context) *apis.FieldError {
	if i == nil {
		return nil
	}

	if i.URL == "" {
		return &apis.FieldError{
			Message: "Target URL is not informed",
			Paths:   []string{"url"},
		}
	}

	if _, err := url.Parse(i.URL); err != nil {
		return &apis.FieldError{
			Message: "Target URL cannot be parsed",
			Paths:   []string{"url"},
			Details: err.Error(),
		}
	}

	return nil
}

type Trigger struct {
	Name    string
	Filters []eventingv1.SubscriptionsAPIFilter
	Targets []Target
}

func (t *Trigger) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if t == nil {
		return nil
	}
	for i, trg := range t.Targets {
		errs = errs.Also(trg.Validate(ctx)).ViaFieldIndex("targets", i)
	}

	return errs.Also(eventingv1.ValidateSubscriptionAPIFiltersList(ctx, t.Filters).ViaField("filters"))
}

type Config struct {
	Ingest   Ingest
	Triggers []Trigger
}

func (c *Config) Validate(ctx context.Context) *apis.FieldError {
	if c == nil {
		return nil
	}

	errs := c.Ingest.Validate(ctx).ViaField("ingest")

	for i, t := range c.Triggers {
		errs = errs.Also(t.Validate(ctx).ViaFieldIndex("triggers", i))
	}

	return errs
}
