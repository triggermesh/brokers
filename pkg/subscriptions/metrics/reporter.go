package metrics

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.uber.org/zap"

	knmetrics "knative.dev/pkg/metrics"

	"github.com/triggermesh/brokers/pkg/common/metrics"
)

const (
	LabelDelivered     = "delivered"
	LabelSentEventType = "sent_type"
	LabelTrigger       = "trigger_name"
)

var (
	sentEventTypeKey  = tag.MustNewKey(LabelSentEventType)
	deliveredEventKey = tag.MustNewKey(LabelDelivered)
	triggerKey        = tag.MustNewKey(LabelTrigger)

	// eventCountM is a counter which records the number of events received
	// by the Broker.
	eventCountM = stats.Int64(
		"trigger/event_count",
		"Number of events sent via Trigger subscription.",
		stats.UnitDimensionless,
	)

	// latencyMs measures the latency in milliseconds for the CloudEvents
	// client methods.
	latencyMs = stats.Float64(
		"trigger/event_latency",
		"The latency in milliseconds for the broker Trigger subscriptions.",
		"ms")
)

func registerStatViews() error {
	tagKeys := []tag.Key{
		triggerKey,
		sentEventTypeKey,
		metrics.ReceivedEventTypeKey,
		deliveredEventKey}

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
	)
}

func initContext(ctx context.Context, triggerName string) (context.Context, error) {
	return tag.New(ctx, tag.Insert(triggerKey, triggerName))
}

type Reporter interface {
	ReportTriggeredEvent(delivered bool, sentType, receivedType string, msLatency float64)
}

// Reporter holds cached metric objects to report ingress metrics.
type reporter struct {
	ctx    context.Context
	logger *zap.SugaredLogger
}

var once sync.Once

// NewReporter retuns a StatReporter for ingested events.
func NewReporter(context context.Context, trigger string) (Reporter, error) {
	r := &reporter{}

	var err error
	once.Do(func() {
		if err = registerStatViews(); err != nil {
			err = fmt.Errorf("error registering OpenCensus stats view: %w", err)
			return
		}
	})

	if err != nil {
		return nil, err
	}

	r.ctx, err = initContext(context, trigger)
	if err != nil {
		return nil, fmt.Errorf("error initializing OpenCensus context with tags: %w", err)
	}

	return r, nil
}

func (r *reporter) ReportTriggeredEvent(delivered bool, sentType, receivedType string, msLatency float64) {
	ctx, err := tag.New(r.ctx,
		tag.Insert(sentEventTypeKey, sentType),
		tag.Insert(metrics.ReceivedEventTypeKey, receivedType),
		tag.Insert(deliveredEventKey, strconv.FormatBool(delivered)),
	)
	if err != nil {
		r.logger.Errorw("error setting tags to OpenCensus context", zap.Error(err))
	}

	knmetrics.Record(ctx, latencyMs.M(msLatency))
	knmetrics.Record(ctx, eventCountM.M(1))
}
