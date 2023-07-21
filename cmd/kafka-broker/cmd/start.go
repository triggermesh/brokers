// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/triggermesh/brokers/pkg/backend/impl/kafka"
	"github.com/triggermesh/brokers/pkg/broker"
	pkgcmd "github.com/triggermesh/brokers/pkg/broker/cmd"
)

type StartCmd struct {
	Kafka kafka.KafkaArgs `embed:"" prefix:"kafka." envprefix:"KAFKA_"`
}

func (s *StartCmd) Validate() error {
	return s.Kafka.Validate()
}

func (c *StartCmd) Run(globals *pkgcmd.Globals) error {
	globals.Logger.Debug("Creating Redis backend client")

	// Use InstanceName as Kafka instance at the consumer group.
	// TODO add namespace to instance name when running at kubernetes
	c.Kafka.Instance = globals.BrokerName
	backend := kafka.New(&c.Kafka, globals.Logger.Named("kafka"))

	b, err := broker.NewInstance(globals, backend)
	if err != nil {
		return err
	}

	return b.Start(globals.Context)
}
