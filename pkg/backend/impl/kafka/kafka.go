package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	krbclient "github.com/jcmturner/gokrb5/v8/client"
	krbconfig "github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/kerberos"

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

	// Client options for creating subscriptions
	kopts []kgo.Opt

	// Client for producing events to Kafka
	client *kgo.Client

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

	s.kopts = []kgo.Opt{
		kgo.SeedBrokers(s.args.Addresses...),

		kgo.ConsumeTopics(s.args.Topic),
		kgo.InstanceID(s.args.Instance),
		kgo.DisableAutoCommit(),
	}

	if ok, _ := s.args.IsGSSAPI(); ok {
		kt, err := keytab.Load(s.args.GssKeyTabPath)
		if err != nil {
			return fmt.Errorf("could not load keytab file: %w", err)
		}

		krbc, err := krbconfig.Load(s.args.GssKerberosConfigPath)
		if err != nil {
			return fmt.Errorf("could not load kerberos config file: %w", err)
		}

		s.kopts = append(s.kopts,
			kgo.SASL(
				kerberos.Auth{
					Client: krbclient.NewWithKeytab(
						s.args.GssPrincipal,
						s.args.GssRealm,
						kt,
						krbc,
					),
					Service: s.args.GssServiceName,
				}.AsMechanismWithClose(),
			))
	}

	client, err := kgo.NewClient(s.kopts...)
	if err != nil {
		return fmt.Errorf("could not create kafka client: %w", err)
	}

	s.client = client

	return s.Probe(ctx)
}

func (s *kafka) Start(ctx context.Context) error {
	s.ctx = ctx
	<-ctx.Done()

	return nil
}

func (s *kafka) Produce(ctx context.Context, event *cloudevents.Event) error {
	// TODO Produce

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
	if s.client == nil {
		return errors.New("kafka client not configured")
	}

	return s.client.Ping(ctx)
}
