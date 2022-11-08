package metrics

import "go.opencensus.io/tag"

const (
	ResourceTypeTriggerMeshBroker = "triggermesh_broker"

	LabelBrokerName = "broker_name"
	LabelUniqueName = "unique_name"
)

var (
	UniqueTagKey = tag.MustNewKey(LabelUniqueName)
)
