// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

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

const (
	// Disconnect timeout
	disconnectTimeout = time.Second * 20

	// Unsubscribe timeout
	unsubscribeTimeout = time.Second * 10
)

func New(args *KafkaArgs, logger *zap.SugaredLogger) backend.Interface {
	return &kafka{
		args:          args,
		logger:        logger,
		disconnecting: false,
		subs:          make(map[string]*subscription),
	}
}

type kafka struct {
	args *KafkaArgs

	// Client options for creating subscriptions
	kopts []kgo.Opt

	// Client for producing events to Kafka
	client *kgo.Client

	// subscription list indexed by the name.
	subs map[string]*subscription

	// Waitgroup that should be used to wait for subscribers
	// before disconnecting.
	wgSubs sync.WaitGroup

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

	// This prevents new subscriptions from being setup
	s.disconnecting = true

	s.mutex.Lock()
	defer s.mutex.Unlock()

	for name := range s.subs {
		s.unsubscribe(name)
	}

	// wait for all subscriptions to finish
	// before returning.
	allSubsFinished := make(chan struct{})
	go func() {
		defer close(allSubsFinished)
		s.wgSubs.Wait()
	}()

	select {
	case <-allSubsFinished:
		// Clean exit.
	case <-time.After(disconnectTimeout):
		// Timed out, some events have not been delivered.
		s.logger.Error(fmt.Sprintf("Disconnection from Redis timed out after %d", disconnectTimeout))
	}

	s.client.Close()
	return nil
}

func (s *kafka) Produce(ctx context.Context, event *cloudevents.Event) error {
	b, err := event.MarshalJSON()
	if err != nil {
		return fmt.Errorf("could not serialize CloudEvent: %w", err)
	}

	r := &kgo.Record{
		Topic: s.args.Topic,
		Value: b,
	}

	var wg sync.WaitGroup
	wg.Add(1)
	if err := s.client.ProduceSync(ctx, r).FirstErr(); err != nil {
		return fmt.Errorf("could not produce CloudEvent to Kafka topic %q: %w", s.args.Topic, err)
	}

	s.logger.Debug(fmt.Sprintf("CloudEvent %s/%s produced to the backend as %d",
		event.Context.GetSource(),
		event.Context.GetID(),
		r.Offset))

	return nil
}

// SubscribeBounded is a variant of the Subscribe function that supports bounded subscriptions.
// It adds the option of using a startId and endId for the replay feature.
func (s *kafka) Subscribe(name string, bounds *broker.TriggerBounds, ccb backend.ConsumerDispatcher, scb backend.SubscriptionStatusChange) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// avoid subscriptions if disconnection is going on
	if s.disconnecting {
		return errors.New("cannot create new subscriptions while disconnecting")
	}

	if _, ok := s.subs[name]; ok {
		return fmt.Errorf("subscription for %q alredy exists", name)
	}

	// TODO calculate bounds
	startOpt, eb, err := boundsResolver(bounds)
	if err != nil {
		return fmt.Errorf("subscription bounds could not be resolved: %w", err)
	}

	kopts := append(s.kopts,
		kgo.ConsumeResetOffset(startOpt),
		kgo.ConsumerGroup(s.args.ConsumerGroupPrefix+"."+name),
		kgo.DisableAutoCommit())

	client, err := kgo.NewClient(kopts...)
	if err != nil {
		return fmt.Errorf("client for subscription could not be created: %w", err)
	}

	// We don't use the parent context but create a new one so that we can control
	// how subscriptions are finished by calling cancel at our will, either when the
	// global context is called, or when unsubscribing.
	ctx, cancel := context.WithCancel(context.Background())

	subs := &subscription{
		instance: s.args.Instance,
		topic:    s.args.Topic,
		// name:     name,
		group:               s.args.ConsumerGroupPrefix,
		checkBoundsExceeded: newExceedBounds(eb),

		trackingEnabled: s.args.TrackingIDEnabled,

		// caller's callback for dispatching events from Kafka.
		ccbDispatch: ccb,

		// cancel function let us control when we want to exit the subscription loop.
		ctx:    ctx,
		cancel: cancel,
		// stoppedCh signals when a subscription has completely finished.
		stoppedCh: make(chan struct{}),

		client: client,
		logger: s.logger,
	}

	s.subs[name] = subs
	s.wgSubs.Add(1)
	subs.start()

	return nil

}

func (s *kafka) Unsubscribe(name string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.unsubscribe(name)
}

func (s *kafka) unsubscribe(name string) {
	sub, ok := s.subs[name]
	if !ok {
		s.logger.Infow("Unsubscribe action was not needed since the subscription did not exist",
			zap.String("name", name))
		return
	}

	// Finish the subscription's context.
	sub.cancel()

	// Wait for the subscription to finish
	select {
	case <-sub.stoppedCh:
		s.logger.Debugw("Graceful shutdown of subscription", zap.String("name", name))

		// Clean exit.
	case <-time.After(unsubscribeTimeout):
		// Timed out, some events have not been delivered.
		s.logger.Errorw(fmt.Sprintf("Unsubscribing from Redis timed out after %d", unsubscribeTimeout),
			zap.String("name", name))
	}

	s.client.Close()
	delete(s.subs, name)
	s.wgSubs.Done()
}

func (s *kafka) Probe(ctx context.Context) error {
	if s.client == nil {
		return errors.New("kafka client not configured")
	}

	return s.client.Ping(ctx)
}

func boundsResolver(bounds *broker.TriggerBounds) (startOp kgo.Offset, eb *endBound, e error) {
	startOp = kgo.NewOffset()

	if bounds == nil {
		startOp = startOp.AfterMilli(time.Now().UnixMilli())
		return
	}

	if start := bounds.ByDate.GetStart(); start != "" {
		st, err := time.Parse(time.RFC3339Nano, start)
		if err != nil {
			e = fmt.Errorf("parsing bounds start date: %w", err)
			return
		}

		startOp = startOp.AfterMilli(st.UnixMilli())
	} else if start := bounds.ByID.GetStart(); start != "" {
		i, err := strconv.ParseInt(start, 10, 64)
		if err != nil {
			e = fmt.Errorf("parsing bounds start id: %w", err)
			return
		}
		startOp = startOp.At(i)
	} else {
		startOp = startOp.AfterMilli(time.Now().UnixMilli())
	}

	endd, endid := bounds.ByDate.GetEnd(), bounds.ByID.GetEnd()
	if endd == "" && endid == "" {
		return
	}

	eb = &endBound{}

	if endid != "" {
		i, err := strconv.ParseInt(endid, 10, 64)
		if err != nil {
			e = fmt.Errorf("parsing bounds end id: %w", err)
			return
		}
		eb.id = i
	}

	en, err := time.Parse(time.RFC3339, endd)
	if err != nil {
		e = fmt.Errorf("parsing bounds end date: %w", err)
		return
	}

	eb.time = &en
	return
}
