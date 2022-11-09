// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
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
)

type Globals struct {
	BrokerConfigPath        string `help:"Path to broker configuration file." env:"BROKER_CONFIG_PATH" default:"/etc/triggermesh/broker.conf"`
	ObservabilityConfigPath string `help:"Path to observability configuration file." env:"OBSERVABILITY_CONFIG_PATH"`
	Port                    int    `help:"HTTP Port to listen for CloudEvents." env:"PORT" default:"8080"`
	BrokerName              string `help:"Instance name. When running at Kubernetes should be set to RedisBroker name" env:"BROKER_NAME" default:"${instance_name}"`

	// Kubernetes parameters
	KubernetesNamespace                  string `help:"Namespace where the broker is running." env:"KUBERNETES_NAMESPACE"`
	KubernetesPod                        string `help:"Pod that runs the broker." env:"KUBERNETES_POD"`
	KubernetesBrokerConfigSecretName     string `help:"Secret object name that contains the broker configuration." env:"KUBERNETES_BROKER_CONFIG_SECRET_NAME"`
	KubernetesBrokerConfigSecretKey      string `help:"Secret object key that contains the broker configuration." env:"KUBERNETES_BROKER_CONFIG_SECRET_KEY"`
	KubernetesObservabilityConfigMapName string `help:"ConfigMap object name that contains the observability configuration." env:"KUBERNETES_OBSERVABILITY_CONFIGMAP_NAME"`

	ObservabilityMetricsDomain string `help:"Domain to be used for some metrics reporters." env:"OBSERVABILITY_METRICS_DOMAIN" default:"triggermesh.io/eventing"`

	Context  context.Context    `kong:"-"`
	Logger   *zap.SugaredLogger `kong:"-"`
	LogLevel zap.AtomicLevel    `kong:"-"`

	// This ID is unique per instance of the broker running.
	InstanceID string `kong:"-"`
}

func (s *Globals) Validate() error {
	msg := []string{}

	if s.BrokerConfigPath == "" &&
		(s.KubernetesBrokerConfigSecretName == "" || s.KubernetesBrokerConfigSecretKey == "") {
		msg = append(msg, "Broker configuration path or ConfigMap must be informed.")
	}

	kubeBroker := false
	if s.KubernetesBrokerConfigSecretName != "" || s.KubernetesBrokerConfigSecretKey != "" {
		kubeBroker = true
	}

	if kubeBroker && (s.KubernetesBrokerConfigSecretName == "" || s.KubernetesBrokerConfigSecretKey == "") {
		msg = append(msg, "Broker configuration for Kubernetes must inform both secret name and key.")
	}

	kubeObservability := false
	if s.KubernetesObservabilityConfigMapName != "" {
		kubeObservability = true
	}

	if kubeObservability && s.ObservabilityConfigPath != "" {
		msg = append(msg, "Observability config must use either a file path or a ConfigMap.")
	}

	if (kubeBroker || kubeObservability) && s.KubernetesNamespace == "" {
		msg = append(msg, "Kubernetes namespace must be informed.")
	}

	if !kubeBroker && !kubeObservability && s.KubernetesNamespace != "" {
		msg = append(msg, "Kubernetes namespace must not be informed when no Secrets/ConfigMaps are watched.")
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

	switch {
	case s.NeedsObservabilityConfigFileWatcher():
		// Read before starting the watcher to use it with the
		// start routines.
		cfg, err = observability.ReadFromFile(s.ObservabilityConfigPath)
		if err != nil || cfg.LoggerCfg == nil {
			log.Printf("Could not appliying provided config: %v", err)
			defaultConfigApplied = true
		}

	case s.NeedsKubernetesObservabilityConfigMap():
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

	s.InstanceID = uuid.New().String()

	// Setup metrics and start exporter.
	s.Context = metrics.InitializeReportingContext(s.BrokerName, s.InstanceID)
	knmetrics.MemStatsOrDie(s.Context)
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
	if cfg == nil {
		return
	}

	m, err := cfg.ToMap()
	if err != nil {
		s.Logger.Errorw("Failed to parse config into map", zap.Error(err))
		return
	}

	err = knmetrics.UpdateExporter(
		s.Context,
		knmetrics.ExporterOptions{
			Domain:         s.ObservabilityMetricsDomain,
			Component:      metricsComponent,
			ConfigMap:      m,
			PrometheusPort: cfg.PrometheusPort,
		},
		s.Logger)

	if err != nil {
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

func (s *Globals) NeedsBrokerConfigFileWatcher() bool {
	// BrokerConfigPath has a default value, it will probably be informed even
	// when Kubernetes secret is being used. For that reason we check if kubernetes
	// is being used for the broker configuration.
	return s.KubernetesBrokerConfigSecretName == ""
}

func (s *Globals) NeedsObservabilityConfigFileWatcher() bool {
	return s.ObservabilityConfigPath != ""
}

func (s *Globals) NeedsFileWatcher() bool {
	return s.NeedsBrokerConfigFileWatcher() || s.NeedsObservabilityConfigFileWatcher()
}

func (s *Globals) NeedsKubernetesBrokerSecret() bool {
	return s.KubernetesBrokerConfigSecretName != "" && s.KubernetesBrokerConfigSecretKey != ""
}

func (s *Globals) NeedsKubernetesObservabilityConfigMap() bool {
	return s.KubernetesObservabilityConfigMapName != ""
}

func (s *Globals) NeedsKubernetesInformer() bool {
	return s.NeedsKubernetesBrokerSecret() || s.NeedsKubernetesObservabilityConfigMap()
}

func (s *Globals) IsKubernetes() bool {
	return s.KubernetesNamespace != "" ||
		s.NeedsKubernetesBrokerSecret() ||
		s.NeedsKubernetesInformer() ||
		s.NeedsKubernetesObservabilityConfigMap()
}
