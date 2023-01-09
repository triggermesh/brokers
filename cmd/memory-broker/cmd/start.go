// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/triggermesh/brokers/pkg/backend/impl/memory"
	"github.com/triggermesh/brokers/pkg/broker"
	pkgcmd "github.com/triggermesh/brokers/pkg/broker/cmd"
)

type StartCmd struct {
	Memory memory.MemoryArgs `embed:"" prefix:"memory." envprefix:"MEMORY_"`
}

func (s *StartCmd) Validate() error {
	return s.Memory.Validate()
}

func (c *StartCmd) Run(globals *pkgcmd.Globals) error {
	globals.Logger.Debug("Creating memory backend client")
	backend := memory.New(&c.Memory, globals.Logger.Named("memory"))

	b, err := broker.NewInstance(globals, backend)
	if err != nil {
		return err
	}

	return b.Start(globals.Context)
}
