package metrics

import (
	"context"
	"time"

	"go.opencensus.io/trace"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	ceclient "github.com/cloudevents/sdk-go/v2/client"

	"github.com/cloudevents/sdk-go/v2/protocol"
)

type opencensusObservabilityService struct {
	reporter Reporter
}

func NewOpenCensusObservabilityService(r Reporter) ceclient.ObservabilityService {
	return opencensusObservabilityService{
		reporter: r,
	}
}

func (o opencensusObservabilityService) InboundContextDecorators() []func(context.Context, binding.Message) context.Context {
	return []func(context.Context, binding.Message) context.Context{tracePropagatorContextDecorator}
}

func (o opencensusObservabilityService) RecordReceivedMalformedEvent(ctx context.Context, err error) {
	o.reporter.ReportNonValidEvent()
}

func (o opencensusObservabilityService) RecordCallingInvoker(ctx context.Context, event *cloudevents.Event) (context.Context, func(errOrResult error)) {
	start := time.Now()
	return ctx, func(errOrResult error) {
		o.reporter.ReportProcessedEvent(
			protocol.IsACK(errOrResult),
			event.Type(),
			float64(time.Since(start)/time.Millisecond))
	}
}

func (o opencensusObservabilityService) RecordSendingEvent(ctx context.Context, event cloudevents.Event) (context.Context, func(errOrResult error)) {
	// Not used at ingest
	return ctx, nil
}

func (o opencensusObservabilityService) RecordRequestEvent(ctx context.Context, event cloudevents.Event) (context.Context, func(errOrResult error, event *cloudevents.Event)) {
	// Not used at ingest
	return ctx, nil
}

func tracePropagatorContextDecorator(ctx context.Context, msg binding.Message) context.Context {
	var messageCtx context.Context
	if mctx, ok := msg.(binding.MessageContext); ok {
		messageCtx = mctx.Context()
	} else if mctx, ok := binding.UnwrapMessage(msg).(binding.MessageContext); ok {
		messageCtx = mctx.Context()
	}

	if messageCtx == nil {
		return ctx
	}
	span := trace.FromContext(messageCtx)
	if span == nil {
		return ctx
	}
	return trace.NewContext(ctx, span)
}
