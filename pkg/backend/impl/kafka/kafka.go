package kafka

import (
	"context"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	"github.com/triggermesh/brokers/pkg/backend"
	"github.com/triggermesh/brokers/pkg/config/broker"
)

func New(args *KafkaArgs, logger *zap.SugaredLogger) backend.Interface {
	return &kafka{
		args:          args,
		logger:        logger,
		disconnecting: false,
		subs:          make(map[string]subscription),
	}
}

type kafka struct {
	args *KafkaArgs

	// subscription list indexed by the name.
	subs map[string]subscription

	// disconnecting is set to avoid setting up new subscriptions
	// when the broker is shutting down.
	disconnecting bool

	ctx    context.Context
	logger *zap.SugaredLogger
	mutex  sync.Mutex
}

func (s *kafka) Info() *backend.Info {
	return &backend.Info{
		Name: "Kafka",
	}
}

func (s *kafka) Init(ctx context.Context) error {
	return nil
}

func (s *kafka) Start(ctx context.Context) error {
	s.ctx = ctx
	<-ctx.Done()

	return nil
}

func (s *kafka) Produce(ctx context.Context, event *cloudevents.Event) error {
	return nil
}

// SubscribeBounded is a variant of the Subscribe function that supports bounded subscriptions.
// It adds the option of using a startId and endId for the replay feature.
func (s *kafka) Subscribe(name string, bounds *broker.TriggerBounds, ccb backend.ConsumerDispatcher, scb backend.SubscriptionStatusChange) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return nil
}

func (s *kafka) Unsubscribe(name string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.unsubscribe(name)
}

func (s *kafka) unsubscribe(name string) {
}

func (s *kafka) Probe(ctx context.Context) error {
	return nil
}
