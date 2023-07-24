// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"

	"github.com/triggermesh/brokers/cmd/kafka-broker/cmd"
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

	hostname, err := os.Hostname()
	if err != nil {
		panic(fmt.Errorf("error retrieving the host name: %w", err))
	}

	kc := kong.Parse(&cli,
		kong.Vars{
			"hostname":  hostname,
			"unique_id": uuid.New().String(),
		})

	err = cli.Initialize()
	if err != nil {
		panic(fmt.Errorf("error initializing: %w", err))
	}
	defer cli.Flush()

	err = kc.Run(&cli.Globals)
	kc.FatalIfErrorf(err)
}
