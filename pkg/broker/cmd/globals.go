// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/triggermesh/brokers/pkg/config/observability"
	"go.uber.org/zap"
)

type Globals struct {
	BrokerConfigPath        string `help:"Path to broker configuration file." env:"BROKER_CONFIG_PATH" default:"/etc/triggermesh/broker.conf"`
	ObservabilityConfigPath string `help:"Path to observability configuration file." env:"OBSERVABILITY_CONFIG_PATH"`
	Port                    int    `help:"HTTP Port to listen for CloudEvents." env:"PORT" default:"8080"`

	Context  context.Context    `kong:"-"`
	Logger   *zap.SugaredLogger `kong:"-"`
	LogLevel zap.AtomicLevel    `kong:"-"`
}

func (s *Globals) Validate() error {
	if s.BrokerConfigPath == "" {
		return errors.New("broker configuration paht must be informed")
	}

	return nil
}

func (s *Globals) Initialize() error {
	var cfg *observability.Config
	var l *zap.Logger
	var err error
	defaultConfigApplied := false

	if s.ObservabilityConfigPath == "" {
		defaultConfigApplied = true
		cfg = observability.DefaultConfig()
	} else {
		// Read before starting the watcher to use it with the
		// start routines.
		cfg, err = observability.ReadFromFile(s.ObservabilityConfigPath)
		if err != nil || cfg.Logging == nil {
			log.Printf("Could not appliying provided config: %v", err)
			defaultConfigApplied = true
			cfg = observability.DefaultConfig()
		}
	}

	// Call build to perform validation of zap configuration.
	l, err = cfg.Logging.Build()
	for {
		if err == nil {
			break
		}
		if defaultConfigApplied {
			return fmt.Errorf("default config failed to be applied due to error: %w", err)
		}

		defaultConfigApplied = true
		cfg = observability.DefaultConfig()
		l, err = cfg.Logging.Build()
	}

	s.LogLevel = cfg.Logging.Level

	s.Logger = l.Sugar()
	s.LogLevel = cfg.Logging.Level

	return nil
}

func (s *Globals) UpdateLevel(cfg *observability.Config) {
	s.Logger.Debugw("Updating logging configuration ...")
	if cfg == nil || cfg.Logging == nil {
		return
	}

	level := cfg.Logging.Level.Level()
	s.Logger.Debugw("Updating logging configuration ...", zap.Any("level", level))
	if s.LogLevel.Level() != level {
		s.Logger.Infof("Updating logging level from %v to %v.", s.LogLevel.Level(), level)
		s.LogLevel.SetLevel(level)
	}
}
