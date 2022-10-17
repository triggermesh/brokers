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

type Target struct {
	URL             string           `json:"url"`
	DeliveryOptions *DeliveryOptions `json:"deliveryOptions,omitempty"`
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

type Filter struct {
	NestedFilter     `json:",inline"`
	ExpressionFilter `json:",inline"`
}

type NestedFilter struct {
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
}

type ExpressionFilter struct {
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

	// CESQL is a CloudEvents SQL expression that will be evaluated to true or false against each CloudEvent.
	//
	// +optional
	CESQL string `json:"cesql,omitempty"`
}

type Trigger struct {
	// Name    string                              `json:"name"`
	// Filters []eventingv1.SubscriptionsAPIFilter `json:"filters,omitempty"`
	Filters []Filter `json:"filters,omitempty"`
	Target  Target   `json:"target"`
}

func (t *Trigger) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if t == nil {
		return nil
	}
	errs = errs.Also(t.Target.Validate(ctx)).ViaField("target")

	return errs.Also(ValidateSubscriptionAPIFiltersList(ctx, t.Filters).ViaField("filters"))
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
