// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package broker

import (
	"context"
	"net/url"

	"knative.dev/pkg/apis"
)

type Ingest struct {
	User     string `json:"user"`
	Password string `json:"password"`
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

type BackoffPolicyType string

const (
	BackoffPolicyConstant    BackoffPolicyType = "constant"
	BackoffPolicyLinear      BackoffPolicyType = "linear"
	BackoffPolicyExponential BackoffPolicyType = "exponential"
)

type DeliveryOptions struct {
	Retry         *int32             `json:"retry,omitempty"`
	BackoffPolicy *BackoffPolicyType `json:"backoffPolicy,omitempty"`

	// BackoffDelay is the delay before retrying.
	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	BackoffDelay  *string `json:"backoffDelay,omitempty"`
	DeadLetterURL *string `json:"deadLetterURL,omitempty"`
}

func (d *DeliveryOptions) Validate(ctx context.Context) (errs *apis.FieldError) {
	if d == nil {
		return
	}

	if d.DeadLetterURL != nil && *d.DeadLetterURL != "" {
		if _, err := url.Parse(*d.DeadLetterURL); err != nil {
			errs = errs.Also(&apis.FieldError{
				Message: "DLS URL cannot be parsed",
				Paths:   []string{"deadLetterURL"},
				Details: err.Error(),
			})
		}
	}

	return
}

type Target struct {
	URL *string `json:"url,,omitempty"`
	// Deprecated, use the trigger's Delivery options instead.
	DeliveryOptions *DeliveryOptions `json:"deliveryOptions,omitempty"`
}

func (i *Target) Validate(ctx context.Context) (errs *apis.FieldError) {
	if i == nil {
		return
	}

	if i.URL != nil && *i.URL != "" {
		if _, err := url.Parse(*i.URL); err != nil {
			errs = errs.Also(&apis.FieldError{
				Message: "Target URL cannot be parsed",
				Paths:   []string{"url"},
				Details: err.Error(),
			})
		}
	}

	return errs.Also(i.DeliveryOptions.Validate(ctx))
}

type Filter struct {
	// All evaluates to true if all the nested expressions evaluate to true.
	// It must contain at least one filter expression.
	//
	// +optional
	All []Filter `json:"all,omitempty"`

	// Any evaluates to true if at least one of the nested expressions evaluates
	// to true. It must contain at least one filter expression.
	//
	// +optional
	Any []Filter `json:"any,omitempty"`

	// Not evaluates to true if the nested expression evaluates to false.
	//
	// +optional
	Not *Filter `json:"not,omitempty"`

	// Exact evaluates to true if the value of the matching CloudEvents
	// attribute matches exactly the String value specified (case-sensitive).
	// Exact must contain exactly one property, where the key is the name of the
	// CloudEvents attribute to be matched, and its value is the String value to
	// use in the comparison. The attribute name and value specified in the filter
	// expression cannot be empty strings.
	//
	// +optional
	Exact map[string]string `json:"exact,omitempty"`

	// Prefix evaluates to true if the value of the matching CloudEvents
	// attribute starts with the String value specified (case-sensitive). Prefix
	// must contain exactly one property, where the key is the name of the
	// CloudEvents attribute to be matched, and its value is the String value to
	// use in the comparison. The attribute name and value specified in the filter
	// expression cannot be empty strings.
	//
	// +optional
	Prefix map[string]string `json:"prefix,omitempty"`

	// Suffix evaluates to true if the value of the matching CloudEvents
	// attribute ends with the String value specified (case-sensitive). Suffix
	// must contain exactly one property, where the key is the name of the
	// CloudEvents attribute to be matched, and its value is the String value to
	// use in the comparison. The attribute name and value specified in the filter
	// expression cannot be empty strings.
	//
	// +optional
	Suffix map[string]string `json:"suffix,omitempty"`
}

// Bounds applied to the trigger that mark the initial and final item to
// be sent from the broker.
type Bounds struct {
	StartID *string `json:"startId"`
	EndDID  *string `json:"endId"`
}

func (b *Bounds) GetStartID() string {
	if b == nil || b.StartID == nil {
		return ""
	}

	return *b.StartID
}

func (b *Bounds) GetEndID() string {
	if b == nil || b.EndDID == nil {
		return ""
	}

	return *b.EndDID
}

type TriggerBounds struct {
	ByID   *Bounds `json:"byId,omitempty"`
	ByDate *Bounds `json:"byDate,omitempty"`
}

type Trigger struct {
	Filters         []Filter         `json:"filters,omitempty"`
	Target          Target           `json:"target"`
	DeliveryOptions *DeliveryOptions `json:"deliveryOptions,omitempty"`
	Bounds          *TriggerBounds   `json:"bounds,omitempty"`
}

// HACK temporary to make the Delivery options move smooth,
// remove this once Target does not host the deliver options.
func (t *Trigger) GetDeliveryOptions() *DeliveryOptions {
	if t.DeliveryOptions != nil {
		return t.DeliveryOptions
	}
	return t.Target.DeliveryOptions
}

func (t *Trigger) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if t == nil {
		return nil
	}
	return errs.Also(t.Target.Validate(ctx)).ViaField("target").
		Also(t.DeliveryOptions.Validate(ctx).ViaField("deliveryOptions")).
		Also(ValidateSubscriptionAPIFiltersList(ctx, t.Filters).ViaField("filters"))
}

type Config struct {
	Ingest   *Ingest            `json:"ingest,omitempty"`
	Triggers map[string]Trigger `json:"triggers"`
}

func (c *Config) Validate(ctx context.Context) *apis.FieldError {
	if c == nil {
		return nil
	}

	errs := c.Ingest.Validate(ctx).ViaField("ingest")

	for k, t := range c.Triggers {
		errs = errs.Also(t.Validate(ctx).ViaFieldKey("triggers", k))
	}

	return errs
}
