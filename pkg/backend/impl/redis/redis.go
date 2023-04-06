// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	goredis "github.com/redis/go-redis/v9"

	"github.com/triggermesh/brokers/pkg/backend"
)

const (
	// Starting point for the consumer group.
	groupStartID = "$"

	// Redis key at the message that contains the CloudEvent.
	ceKey = "ce"

	// Disconnect timeout
	disconnectTimeout = time.Second * 20

	// Unsubscribe timeout
	unsubscribeTimeout = time.Second * 10
)

func New(args *RedisArgs, logger *zap.SugaredLogger) backend.Interface {
	return &redis{
		args:          args,
		logger:        logger,
		disconnecting: false,
		subs:          make(map[string]subscription),
	}
}

type redis struct {
	args *RedisArgs

	client goredis.Cmdable
	// Redis' Cmdable does not include the conneciton operation
	// functions, we keep track of closing via this field.
	clientClose func() error

	// subscription list indexed by the name.
	subs map[string]subscription
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

func (s *redis) Info() *backend.Info {
	return &backend.Info{
		Name: "Redis",
	}
}

func (s *redis) Init(ctx context.Context) error {
	var tlscfg *tls.Config
	if s.args.TLSEnabled {
		tlscfg = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: s.args.TLSSkipVerify,
		}

		roots := x509.NewCertPool()
		if s.args.TLSCACertificate != "" {
			if ok := roots.AppendCertsFromPEM([]byte(s.args.TLSCACertificate)); !ok {
				return errors.New("not valid CA Cert format")
			}
		}

		tlscfg.RootCAs = roots

		if s.args.TLSCertificate != "" {
			cert, err := tls.X509KeyPair([]byte(s.args.TLSCertificate), []byte(s.args.TLSKey))
			if err != nil {
				return fmt.Errorf("TLS key pair should be PEM formatted: %w", err)
			}
			tlscfg.Certificates = append(tlscfg.Certificates, cert)
		}
	}

	if len(s.args.ClusterAddresses) != 0 {
		s.logger.Info("Cluster client")
		clusterclient := goredis.NewClusterClient(&goredis.ClusterOptions{
			Addrs:     s.args.ClusterAddresses,
			Username:  s.args.Username,
			Password:  s.args.Password,
			TLSConfig: tlscfg,
		})

		s.clientClose = clusterclient.Close
		s.client = clusterclient
	} else {
		client := goredis.NewClient(&goredis.Options{
			Addr:      s.args.Address,
			Username:  s.args.Username,
			Password:  s.args.Password,
			DB:        s.args.Database,
			TLSConfig: tlscfg,
		})

		s.clientClose = client.Close
		s.client = client
	}

	return s.Probe(ctx)
}

func (s *redis) Start(ctx context.Context) error {
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

	return s.clientClose()
}

func (s *redis) Produce(ctx context.Context, event *cloudevents.Event) error {
	b, err := event.MarshalJSON()
	if err != nil {
		return fmt.Errorf("could not serialize CloudEvent: %w", err)
	}

	args := &goredis.XAddArgs{
		Stream: s.args.Stream,
		Values: map[string]interface{}{ceKey: b},
	}

	if s.args.StreamMaxLen != 0 {
		args.MaxLen = int64(s.args.StreamMaxLen)
		args.Approx = true
	}

	res := s.client.XAdd(ctx, args)

	id, err := res.Result()
	if err != nil {
		return fmt.Errorf("could not produce CloudEvent to backend: %w", err)
	}

	s.logger.Debug(fmt.Sprintf("CloudEvent %s/%s produced to the backend as %s",
		event.Context.GetSource(),
		event.Context.GetID(),
		id))

	return nil
}

func (s *redis) Subscribe(name string, ccb backend.ConsumerDispatcher) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// avoid subscriptions if disconnection is going on
	if s.disconnecting {
		return errors.New("cannot create new subscriptions while disconnecting")
	}

	if _, ok := s.subs[name]; ok {
		return fmt.Errorf("subscription for %q alredy exists", name)
	}

	// Create the consumer group for this subscription.
	group := s.args.Group + "." + name
	res := s.client.XGroupCreateMkStream(s.ctx, s.args.Stream, group, groupStartID)
	_, err := res.Result()
	if err != nil {
		// Ignore errors when the group already exists.
		if !strings.HasPrefix(err.Error(), "BUSYGROUP") {
			return err
		}
		s.logger.Debug("Consumer group already exists", zap.String("group", group))
	}

	// We don't use the parent context but create a new one so that we can control
	// how subscriptions are finished by calling cancel at our will, either when the
	// global context is called, or when unsubscribing.
	ctx, cancel := context.WithCancel(context.Background())

	subs := subscription{
		instance: s.args.Instance,
		stream:   s.args.Stream,
		name:     name,
		group:    group,

		// caller's callback for dispatching events from Redis.
		ccbDispatch: ccb,

		// cancel function let us control when we want to exit the subscription loop.
		ctx:    ctx,
		cancel: cancel,
		// stoppedCh signals when a subscription has completely finished.
		stoppedCh: make(chan struct{}),

		client: s.client,
		logger: s.logger,
	}

	s.subs[name] = subs
	s.wgSubs.Add(1)
	subs.start()

	return nil
}

func (s *redis) Unsubscribe(name string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.unsubscribe(name)
}

// unsubscribe is not thread safe, caller should acquire
// the object's lock.
func (s *redis) unsubscribe(name string) {
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

	delete(s.subs, name)
	s.wgSubs.Done()
}

func (s *redis) Probe(ctx context.Context) error {
	switch s.args.ProbeCommand {
	case "CLIENTID":
		res := s.client.ClientID(ctx)
		id, err := res.Result()
		if err != nil {
			return fmt.Errorf("failed probing Redis, retrieving client ID: %w", err)
		}

		s.logger.Debugw("Probing redis", zap.Int64("client_id", id))

	case "XINFOSTREAM":
		res := s.client.XInfoStream(ctx, s.args.Stream)
		_, err := res.Result()
		if err != nil && !strings.Contains(err.Error(), "no such key") {
			return fmt.Errorf("failed probing Redis, retrieving stream %s info: %w", s.args.Stream, err)
		}

		s.logger.Debugw("Probing redis")

	case "NONE":
	default:
		return fmt.Errorf("not a valid probing option: %s", s.args.ProbeCommand)
	}

	return nil
}
