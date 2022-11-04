// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kong"

	"github.com/triggermesh/brokers/cmd/memory-broker/cmd"
	pkgcmd "github.com/triggermesh/brokers/pkg/broker/cmd"
)

type cli struct {
	pkgcmd.Globals

	Start cmd.StartCmd `cmd:"" help:"Starts the TriggerMesh broker."`
}

func main() {
	cli := cli{
		Globals: pkgcmd.Globals{
			Context: context.Background(),
		},
	}

	instance, err := os.Hostname()
	if err != nil {
		panic(fmt.Errorf("error retrieving the host name: %w", err))
	}

	kc := kong.Parse(&cli,
		kong.Vars{
			"instance_name": instance,
		})

	err = cli.Initialize()
	if err != nil {
		panic(fmt.Errorf("error initializing: %w", err))
	}
	defer cli.Flush()

	err = kc.Run(&cli.Globals)
	kc.FatalIfErrorf(err)
}
