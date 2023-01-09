// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/rickb777/date/period"
	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	knmetrics "knative.dev/pkg/metrics"

	"github.com/triggermesh/brokers/pkg/common/metrics"
	"github.com/triggermesh/brokers/pkg/config/observability"
)

const (
	metricsComponent = "broker"

	defaultBrokerConfigPath = "/etc/triggermesh/broker.conf"
)

type ConfigMethod int

const (
	ConfigMethodUnknown = iota
	ConfigMethodFileWatcher
	ConfigMethodFilePoller
	ConfigMethodKubernetesSecretMapWatcher
	ConfigMethodInline
)

type Globals struct {
	BrokerConfigPath        string `help:"Path to broker configuration file." env:"BROKER_CONFIG_PATH" default:"/etc/triggermesh/broker.conf"`
	ObservabilityConfigPath string `help:"Path to observability configuration file." env:"OBSERVABILITY_CONFIG_PATH"`
	Port                    int    `help:"HTTP Port to listen for CloudEvents." env:"PORT" default:"8080"`
	BrokerName              string `help:"Broker instance name. When running at Kubernetes should be set to RedisBroker name" env:"BROKER_NAME" default:"${hostname}"`

	// Config Polling is an alternative to the default file watcher for config files.
	ConfigPollingPeriod string `help:"Period for polling the configuration files using ISO8601. A zero duration disables configuration by polling." env:"CONFIG_POLLING_PERIOD" default:"PT0S"`

	// Inline Configuration
	BrokerConfig        string `help:"JSON representation of broker configuration." env:"BROKER_CONFIG"`
	ObservabilityConfig string `help:"JSON representation of observability configuration." env:"OBSERVABILITY_CONFIG"`

	// Kubernetes parameters
	KubernetesNamespace                  string `help:"Namespace where the broker is running." env:"KUBERNETES_NAMESPACE"`
	KubernetesBrokerConfigSecretName     string `help:"Secret object name that contains the broker configuration." env:"KUBERNETES_BROKER_CONFIG_SECRET_NAME"`
	KubernetesBrokerConfigSecretKey      string `help:"Secret object key that contains the broker configuration." env:"KUBERNETES_BROKER_CONFIG_SECRET_KEY"`
	KubernetesObservabilityConfigMapName string `help:"ConfigMap object name that contains the observability configuration." env:"KUBERNETES_OBSERVABILITY_CONFIGMAP_NAME"`

	ObservabilityMetricsDomain string `help:"Domain to be used for some metrics reporters." env:"OBSERVABILITY_METRICS_DOMAIN" default:"triggermesh.io/eventing"`

	Context       context.Context    `kong:"-"`
	Logger        *zap.SugaredLogger `kong:"-"`
	LogLevel      zap.AtomicLevel    `kong:"-"`
	PollingPeriod time.Duration      `kong:"-"`
	ConfigMethod  ConfigMethod       `kong:"-"`
}

func (s *Globals) Validate() error {
	msg := []string{}

	// We need to sort out if ConfigPollingPeriod is not 0 before
	// finding out the config method
	if s.ConfigPollingPeriod != "" {
		p, err := period.Parse(s.ConfigPollingPeriod)
		if err != nil {
			msg = append(msg, fmt.Sprintf("Polling frequency is not an ISO8601 duration: %v", err))
		} else {
			s.PollingPeriod = p.DurationApprox()
		}
	}

	// Broker config must be configured
	if s.BrokerConfigPath == "" &&
		(s.KubernetesBrokerConfigSecretName == "" || s.KubernetesBrokerConfigSecretKey == "") &&
		s.BrokerConfig == "" {
		msg = append(msg, "Broker configuration path, Kubernetes Secret, or inline configuration must be informed.")
	}

	switch {
	case s.KubernetesBrokerConfigSecretName != "" || s.KubernetesBrokerConfigSecretKey != "":
		s.ConfigMethod = ConfigMethodKubernetesSecretMapWatcher

		if s.KubernetesNamespace == "" {
			msg = append(msg, "Kubernetes namespace must be informed.")
		}

		if s.KubernetesBrokerConfigSecretName == "" || s.KubernetesBrokerConfigSecretKey == "" {
			msg = append(msg, "Broker configuration for Kubernetes must inform both secret name and key.")
		}

		// Local file config path should be either empty or the default, which is considered empty
		// when Kubernetes configuration is informed.
		if s.BrokerConfigPath != "" && s.BrokerConfigPath != defaultBrokerConfigPath {
			msg = append(msg, "Cannot use Broker file for configuration when a Kubernetes Secret is used for the broker.")
		}

		// Local file config path should be either empty or the default, which is considered empty
		// when Kubernetes configuration is informed.
		if s.ObservabilityConfigPath != "" {
			msg = append(msg, "Local file observability configuration cannot be used along with the Kubernetes Secret configuration.")
		}

		if s.BrokerConfig != "" || s.ObservabilityConfig != "" {
			msg = append(msg, "Inline config cannot be used along with the Kubernetes Secret configuration.")
		}

	case s.BrokerConfig != "":
		// Local file config path should be either empty or the default, which is considered empty
		// when Kubernetes configuration is informed.
		if s.BrokerConfigPath != "" && s.BrokerConfigPath != defaultBrokerConfigPath {
			msg = append(msg, "Inline config cannot be used along with local file configuration.")
			break
		}

		s.ConfigMethod = ConfigMethodInline

	case s.BrokerConfigPath != "":
		if s.PollingPeriod == 0 {
			s.ConfigMethod = ConfigMethodFileWatcher
		} else {
			s.ConfigMethod = ConfigMethodFilePoller
		}

		if s.KubernetesBrokerConfigSecretName != "" || s.KubernetesBrokerConfigSecretKey != "" {
			msg = append(msg, "Cannot inform Broker Secret and File for broker configuration.")
		}

		if s.KubernetesObservabilityConfigMapName != "" {
			msg = append(msg, "Cannot inform Observability ConfigMap when a file is used for broker configuration.")
		}

		if s.KubernetesNamespace != "" {
			msg = append(msg, "Kubernetes namespace must not be informed when local File configuration is used.")
		}

		if s.BrokerConfig != "" || s.ObservabilityConfig != "" {
			msg = append(msg, "Inline config cannot be used along with local file configuration.")
		}

	default:
		msg = append(msg, "Either Kubernetes Secret or local file configuration must be informed.")
	}

	if len(msg) != 0 {
		s.ConfigMethod = ConfigMethodUnknown
		return fmt.Errorf(strings.Join(msg, " "))
	}

	return nil
}

func (s *Globals) Initialize() error {
	var cfg *observability.Config
	var l *zap.Logger
	defaultConfigApplied := false
	var err error

	switch {
	case s.ObservabilityConfigPath != "":
		// Read before starting the watcher to use it with the
		// start routines.
		cfg, err = observability.ReadFromFile(s.ObservabilityConfigPath)
		if err != nil || cfg.LoggerCfg == nil {
			log.Printf("Could not appliying provided config: %v", err)
			defaultConfigApplied = true
		}

	case s.ObservabilityConfig != "":
		data := map[string]string{}
		err = json.Unmarshal([]byte(s.ObservabilityConfig), &data)
		if err != nil {
			log.Printf("Could not appliying provided config: %v", err)
			defaultConfigApplied = true
			break
		}

		cfg, err = observability.ParseFromMap(data)
		if err != nil || cfg.LoggerCfg == nil {
			log.Printf("Could not appliying provided config: %v", err)
			defaultConfigApplied = true
		}

	case s.KubernetesObservabilityConfigMapName != "":
		kc, err := client.New(config.GetConfigOrDie(), client.Options{})
		if err != nil {
			return err
		}

		cm := &corev1.ConfigMap{}
		var lastErr error

		if err := wait.PollImmediate(1*time.Second, 5*time.Second, func() (bool, error) {
			lastErr = kc.Get(s.Context, client.ObjectKey{
				Namespace: s.KubernetesNamespace,
				Name:      s.KubernetesObservabilityConfigMapName,
			}, cm)

			return lastErr == nil || apierrors.IsNotFound(lastErr), nil
		}); err != nil {
			log.Printf("Could not retrieve observability ConfigMap %q: %v",
				s.KubernetesObservabilityConfigMapName, err)
			defaultConfigApplied = true
		}

		cfg, err = observability.ParseFromMap(cm.Data)
		if err != nil || cfg.LoggerCfg == nil {
			log.Printf("Could not apply provided config from ConfigMap %q: %v",
				s.KubernetesObservabilityConfigMapName, err)
			defaultConfigApplied = true
		}

	default:
		log.Print("Applying default configuration")
		defaultConfigApplied = true
	}

	if defaultConfigApplied {
		cfg = observability.DefaultConfig()
	}

	// Call build to perform validation of zap configuration.
	l, err = cfg.LoggerCfg.Build()
	for {
		if err == nil {
			break
		}
		if defaultConfigApplied {
			return fmt.Errorf("default config failed to be applied due to error: %w", err)
		}

		defaultConfigApplied = true
		cfg = observability.DefaultConfig()
		l, err = cfg.LoggerCfg.Build()
	}

	s.LogLevel = cfg.LoggerCfg.Level

	s.Logger = l.Sugar()
	s.LogLevel = cfg.LoggerCfg.Level

	// Setup metrics and start exporter.
	knmetrics.MemStatsOrDie(s.Context)
	s.Context = metrics.InitializeReportingContext(s.Context, s.BrokerName)
	s.UpdateMetricsOptions(cfg)

	return nil
}

func (s *Globals) Flush() {
	if s.Logger != nil {
		_ = s.Logger.Sync()
	}
	knmetrics.FlushExporter()
}

func (s *Globals) UpdateMetricsOptions(cfg *observability.Config) {
	s.Logger.Debugw("Updating metrics configuration.")
	if cfg == nil || cfg.MetricsConfig == nil {
		return
	}

	m, err := cfg.ToMap()
	if err != nil {
		s.Logger.Errorw("Failed to parse config into map", zap.Error(err))
		return
	}

	if err = knmetrics.UpdateExporter(
		s.Context,
		knmetrics.ExporterOptions{
			Domain:         s.ObservabilityMetricsDomain,
			Component:      metricsComponent,
			ConfigMap:      m,
			PrometheusPort: cfg.PrometheusPort,
		},
		s.Logger); err != nil {
		s.Logger.Errorw("failed to update metrics exporter", zap.Error(err))
	}
}

func (s *Globals) UpdateLogLevel(cfg *observability.Config) {
	s.Logger.Debugw("Updating logging configuration.")
	if cfg == nil || cfg.LoggerCfg == nil {
		return
	}

	level := cfg.LoggerCfg.Level.Level()
	s.Logger.Debugw("Updating logging level", zap.Any("level", level))
	if s.LogLevel.Level() != level {
		s.Logger.Infof("Updating logging level from %v to %v.", s.LogLevel.Level(), level)
		s.LogLevel.SetLevel(level)
	}
}
