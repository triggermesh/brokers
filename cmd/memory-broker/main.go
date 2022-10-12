// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"

	"github.com/alecthomas/kong"
	"go.uber.org/zap"

	"github.com/triggermesh/brokers/cmd/memory-broker/cmd"
	pkgcmd "github.com/triggermesh/brokers/pkg/broker/cmd"
)

type cli struct {
	pkgcmd.Globals

	Start cmd.StartCmd `cmd:"" help:"Starts the TriggerMesh broker."`
}

func main() {

	// TODO configure logger
	zl, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	cli := cli{
		Globals: pkgcmd.Globals{
			Logger:  zl.Sugar(),
			Context: context.Background(),
		},
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
