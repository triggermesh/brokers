// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	goredis "github.com/go-redis/redis/v9"

	"github.com/triggermesh/brokers/pkg/backend"
)

const (
	// Starting point for the consumer group.
	groupStartID = "$"

	// Redis key at the message that contains the CloudEvent.
	ceKey = "ce"
)

func New(logger *zap.Logger, args *RedisArgs) backend.Interface {
	return &redis{
		args:   args,
		logger: logger,
	}
}

type redis struct {
	args *RedisArgs

	client *goredis.Client
	logger *zap.Logger
}

func (s *redis) Info() *backend.Info {
	return &backend.Info{
		Name: "Redis",
	}
}

func (s *redis) Init(ctx context.Context) error {
	s.client = goredis.NewClient(&goredis.Options{
		Addr:     s.args.Address,
		Password: s.args.Password,
		DB:       s.args.Database,
	})

	// Create the consumer group
	res := s.client.XGroupCreateMkStream(ctx, s.args.Stream, s.args.Group, groupStartID)
	_, err := res.Result()
	if err != nil {
		// Ignore errors when the group already exists.
		if !strings.HasPrefix(err.Error(), "BUSYGROUP") {
			return err
		}
		s.logger.Debug("Consumer group already exists")
	}

	return nil
}

func (s *redis) Disconnect() error {
	// TODO signal start routine to stop reading from the consumer group.
	return s.client.Close()
}

func (s *redis) Produce(ctx context.Context, event *cloudevents.Event) error {
	b, err := event.MarshalJSON()
	if err != nil {
		return fmt.Errorf("could not serialize CloudEvent: %w", err)
	}

	res := s.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: s.args.Stream,
		Values: map[string]interface{}{ceKey: b},
	})

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

func (s *redis) Start(ctx context.Context, ccb backend.ConsumerDispatcher) {
	// start reading all pending messages
	id := "0"

	for {
		streams, err := s.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    s.args.Group,
			Consumer: s.args.Instance,
			Streams:  []string{s.args.Stream, id},
			Count:    1,
			Block:    time.Hour,
			NoAck:    false,
		}).Result()

		if err != nil {
			// Ignore errors when the blocking period ends without
			// receiving any event.
			if !strings.HasSuffix(err.Error(), "i/o timeout") {
				s.logger.Error("could not read CloudEvent from consumer group", zap.Error(err))
			}
			continue
		}

		if len(streams) != 1 {
			s.logger.Error("unexpected number of streams read", zap.Any("streams", streams))
			continue
		}

		// If we are processing pending messages from Redis and we reach
		// EOF, switch to reading new messages.
		if len(streams[0].Messages) == 0 && id != ">" {
			id = ">"
		}

		for _, msg := range streams[0].Messages {
			ce := &cloudevents.Event{}
			for k, v := range msg.Values {
				if k != ceKey {
					s.logger.Debug(fmt.Sprintf("Ignoring non expected key at message from backend: %s", k))
					continue
				}

				if err = ce.UnmarshalJSON([]byte(v.(string))); err != nil {
					s.logger.Error("Could not unmarshal CloudEvent from Redis", zap.Error(err))
					continue
				}
			}

			// If there was no valid CE in the message ACK so that we do not receive it again.
			if ce.ID() == "" {
				s.logger.Warn(fmt.Sprintf("Removing non valid message from backend: %s", msg.ID))
				s.ack(ctx, msg.ID)
				continue
			}

			ce.Context.SetExtension("tmbackendid", msg.ID)

			go func() {
				ccb(ce)
				id := ce.Extensions()["tmbackendid"].(string)

				if err := s.ack(ctx, id); err != nil {
					s.logger.Error(fmt.Sprintf("could not ACK the Redis message %s containing CloudEvent %s", id, ce.Context.GetID()),
						zap.Error(err))
				}
			}()

			// If we are processing pending messages the ACK might take a
			// while to be sent. We need to set the message ID so that the
			// next requested element is not any of the pending being processed.
			if id != ">" {
				id = msg.ID
			}
		}

	}
}

func (s *redis) ack(ctx context.Context, id string) error {
	res := s.client.XAck(ctx, s.args.Stream, s.args.Group, id)
	_, err := res.Result()
	return err
}

func (s *redis) Probe(ctx context.Context) error {
	res := s.client.ClientID(ctx)
	id, err := res.Result()
	s.logger.Debug(fmt.Sprintf("DEBUG id is %d", id))
	return err
}
