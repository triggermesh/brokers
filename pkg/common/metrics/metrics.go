// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"sync"

	"go.opencensus.io/resource"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics/metricskey"
)

const (
	ResourceTypeTriggerMeshBroker = "triggermesh_broker"

	LabelBrokerName        = "broker_name"
	LabelUniqueName        = "unique_name"
	LabelReceivedEventType = "received_type"
)

var (
	reportingContext context.Context
	once             sync.Once

	ReceivedEventTypeKey = tag.MustNewKey(LabelReceivedEventType)
)

func InitializeReportingContext(brokerName, instanceID string) context.Context {
	once.Do(func() {
		reportingContext = metricskey.WithResource(context.Background(), resource.Resource{
			Type: ResourceTypeTriggerMeshBroker,
			Labels: map[string]string{
				LabelBrokerName: brokerName,
				LabelUniqueName: instanceID,
			},
		})
	})

	return reportingContext
}
