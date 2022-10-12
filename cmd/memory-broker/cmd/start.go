// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/triggermesh/brokers/pkg/backend/impl/memory"
	pkgcmd "github.com/triggermesh/brokers/pkg/cmd"
)

type StartCmd struct {
	Memory memory.MemoryArgs `embed:"" prefix:"memory." envprefix:"MEMORY_"`
}

func (c *StartCmd) Run(globals *pkgcmd.Globals) error {
	globals.Logger.Debug("Creating memory backend client")
	b := memory.New(&c.Memory, globals.Logger.Named("memory"))

	return pkgcmd.Run(globals, b)
}
