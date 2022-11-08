package metrics

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"go.opencensus.io/resource"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"

	knmetrics "knative.dev/pkg/metrics"
	"knative.dev/pkg/metrics/metricskey"

	"github.com/triggermesh/brokers/pkg/common/metrics"
)

const (
	// LabelIngestedClass is the label that marks a CloudEvents as ingested in the broker.
	LabelIngestedClass = "ingested"
	LabelEventType     = "event_type"
)

var (
	eventTypeKey     = tag.MustNewKey(LabelEventType)
	ingestedEventKey = tag.MustNewKey(LabelIngestedClass)

	// eventCountM is a counter which records the number of events received
	// by the Broker.
	eventCountM = stats.Int64(
		"ingest/event_count",
		"Number of events received by a Broker ingestion.",
		stats.UnitDimensionless,
	)

	// latencyMs measures the latency in milliseconds for the CloudEvents
	// client methods.
	latencyMs = stats.Float64(
		"ingest/event_latency",
		"The latency in milliseconds for the broker CloudEvents ingestion.",
		"ms")

	// rejectedCountM is a counter which records the number of requests that
	// could not be processed as events.
	rejectedCountM = stats.Int64(
		"ingest/rejected_count",
		"Number of requests rejected by the Broker ingestion.",
		stats.UnitDimensionless,
	)
)

func registerStatViews() error {
	tagKeys := []tag.Key{
		eventTypeKey,
		ingestedEventKey,
		metrics.UniqueTagKey}

	// Create view to see our measurements.
	return knmetrics.RegisterResourceView(
		&view.View{
			Name:        latencyMs.Name(),
			Description: latencyMs.Description(),
			Measure:     latencyMs,
			Aggregation: view.Distribution(0, .01, .1, 1, 10, 100, 1000, 10000),
			TagKeys:     tagKeys,
		},
		&view.View{
			Name:        eventCountM.Name(),
			Description: eventCountM.Description(),
			Measure:     eventCountM,
			Aggregation: view.Count(),
			TagKeys:     tagKeys,
		},
		&view.View{
			Name:        rejectedCountM.Name(),
			Description: rejectedCountM.Description(),
			Measure:     rejectedCountM,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{metrics.UniqueTagKey},
		},
	)
}

func initContext(brokerName, instanceID string) (context.Context, error) {
	ctx := metricskey.WithResource(context.Background(), resource.Resource{
		Type: metrics.ResourceTypeTriggerMeshBroker,
		Labels: map[string]string{
			metrics.LabelBrokerName: brokerName,
		},
	})

	return tag.New(ctx, tag.Insert(metrics.UniqueTagKey, instanceID))
}

// Reporter represents a running latency counter. When Error or OK are
// called, the latency is calculated. Error or OK are only allowed to
// be called once.
type Reporter interface {
	ReportProcessedEvent(ingested bool, eventType string, msLatency float64)
	ReportNonValidEvent()
}

// Reporter holds cached metric objects to report ingress metrics.
type reporter struct {
	ctx    context.Context
	logger *zap.SugaredLogger
}

var once sync.Once

// NewReporter retuns a StatReporter for ingested events.
// It must be informed the broker name and a unique instanceID per running instance.
func NewReporter(brokerName, instanceID string) (Reporter, error) {
	r := &reporter{}

	var err error
	once.Do(func() {
		if err = registerStatViews(); err != nil {
			err = fmt.Errorf("error registering OpenCensus stats view: %w", err)
			return
		}

		r.ctx, err = initContext(brokerName, instanceID)
		if err != nil {
			err = fmt.Errorf("error initializing OpenCensus context with tags: %w", err)
		}
	})

	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *reporter) ReportProcessedEvent(ingested bool, eventType string, msLatency float64) {
	ctx, err := tag.New(r.ctx,
		tag.Insert(eventTypeKey, eventType),
		tag.Insert(ingestedEventKey, strconv.FormatBool(ingested)),
	)
	if err != nil {
		r.logger.Errorw("error setting tags to OpenCensus context", zap.Error(err))
	}

	knmetrics.Record(ctx, latencyMs.M(msLatency))
	knmetrics.Record(ctx, eventCountM.M(1))
}

func (r *reporter) ReportNonValidEvent() {
	knmetrics.Record(r.ctx, rejectedCountM.M(1))
}
