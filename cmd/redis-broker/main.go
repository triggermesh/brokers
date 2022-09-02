// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"

	"github.com/alecthomas/kong"
	"go.uber.org/zap"

	"github.com/triggermesh/brokers/cmd/redis-broker/cmd"
)

type Gateway struct {
	cmd.Globals

	Start cmd.StartCmd `cmd:"" help:"Starts the TriggerMesh gateway."`
}

func main() {
	g := cmd.Globals{}

	// TODO configure logger
	zl, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	g.SetLogger(zl)
	g.SetContext(context.Background())

	cli := Gateway{
		Globals: g,
	}

	instance, err := os.Hostname()
	if err != nil {
		zl.Panic("error retrieving the host name", zap.Error(err))
	}

	kc := kong.Parse(&cli,
		kong.Vars{
			"instance_name": instance,
		})
	err = kc.Run(&cli.Globals)
	kc.FatalIfErrorf(err)
}