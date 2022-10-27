// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/triggermesh/brokers/pkg/config/observability"
	"go.uber.org/zap"
)

type Globals struct {
	BrokerConfigPath        string `help:"Path to broker configuration file." env:"BROKER_CONFIG_PATH" default:"/etc/triggermesh/broker.conf"`
	ObservabilityConfigPath string `help:"Path to observability configuration file." env:"OBSERVABILITY_CONFIG_PATH"`
	Port                    int    `help:"HTTP Port to listen for CloudEvents." env:"PORT" default:"8080"`

	BrokerConfigKubernetesSecretName        string `help:"Secret object name that contains the broker configuration." env:"BROKER_CONFIG_KUBERNETES_SECRET_NAME"`
	BrokerConfigKubernetesSecretKey         string `help:"Secret object key that contains the broker configuration." env:"BROKER_CONFIG_KUBERNETES_SECRET_KEY"`
	ObservabilityConfigKubernetesSecretName string `help:"Secret object name that contains the observability configuration." env:"OBSERVABILITY_CONFIG_KUBERNETES_SECRET_NAME"`
	ObservabilityConfigKubernetesSecretKey  string `help:"Secret object key that contains the observability configuration." env:"OBSERVABILITY_CONFIG_KUBERNETES_SECRET_KEY"`

	Context  context.Context    `kong:"-"`
	Logger   *zap.SugaredLogger `kong:"-"`
	LogLevel zap.AtomicLevel    `kong:"-"`
}

func (s *Globals) Validate() error {
	msg := []string{}

	if s.BrokerConfigPath == "" &&
		(s.BrokerConfigKubernetesSecretName == "" || s.BrokerConfigKubernetesSecretKey == "") {
		msg = append(msg, "Broker configuration paht must be informed.")
	}

	if (s.BrokerConfigKubernetesSecretName != "" && s.BrokerConfigKubernetesSecretKey == "") ||
		(s.BrokerConfigKubernetesSecretName == "" && s.BrokerConfigKubernetesSecretKey != "") {
		msg = append(msg, "Broker configuration for Kubernetes must inform both secret name and key.")
	}

	if (s.ObservabilityConfigKubernetesSecretName != "" && s.ObservabilityConfigKubernetesSecretKey == "") ||
		(s.ObservabilityConfigKubernetesSecretName == "" && s.ObservabilityConfigKubernetesSecretKey != "") {
		msg = append(msg, "Observability configuration for Kubernetes must inform both secret name and key.")
	}

	if len(msg) != 0 {
		return fmt.Errorf(strings.Join(msg, " "))
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

func (s *Globals) NeedsFileWatcher() bool {
	// BrokerConfigPath has a default value, it will probably be informed even
	// when Kubernetes secret is being used. For that reason we check if kubernetes
	// is being used for the broker configuration.
	return s.BrokerConfigKubernetesSecretName == "" || s.ObservabilityConfigPath != ""
}

func (s *Globals) NeedsKubernetesInformer() bool {
	return s.BrokerConfigKubernetesSecretName != "" || s.ObservabilityConfigKubernetesSecretName != ""
}
