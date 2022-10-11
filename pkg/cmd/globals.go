// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"

	"go.uber.org/zap"
)

type Globals struct {
	ConfigPath string `help:"Path to configuration file." env:"CONFIG_PATH" default:"/etc/triggermesh/broker.conf"`

	Context context.Context    `kong:"-"`
	Logger  *zap.SugaredLogger `kong:"-"`
}

func (s *Globals) Validate() error {
	if s.ConfigPath == "" {
		return errors.New("broker configuration paht must be informed")
	}

	return nil
}
