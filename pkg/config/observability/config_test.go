// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestParse(t *testing.T) {
	info := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	cases := map[string]struct {
		content        string
		expectedConfig Config
	}{
		"simple logger config": {
			content: `
zap-logger-config: |
  {
    "level": "info"
  }

`, expectedConfig: Config{
				LoggerCfg: &zap.Config{
					Level:       info,
					Development: false,
				},
				MetricsConfig: &MetricsConfig{},
			},
		},
		"full logger config": {
			content: `
zap-logger-config: |
  {
    "level": "info",
    "development": true,
    "outputPaths": ["stdout"],
    "errorOutputPaths": ["stderr"],
    "encoding": "json",
    "encoderConfig": {
      "timeKey": "timestamp",
      "levelKey": "severity",
      "nameKey": "logger",
      "callerKey": "caller",
      "messageKey": "message",
      "stacktraceKey": "stacktrace",
      "lineEnding": "",
      "levelEncoder": "",
      "timeEncoder": "iso8601",
      "durationEncoder": "",
      "callerEncoder": ""
    }
  }
`, expectedConfig: Config{
				LoggerCfg: &zap.Config{
					Level:       info,
					Development: true,
				},
				MetricsConfig: &MetricsConfig{},
			},
		},
		"metrics config": {
			content: `
zap-logger-config: |
  {
    "level": "info"
  }
metrics.backend-destination: prometheus
metrics.reporting-period-seconds: 5
metrics.prometheus-port: 9092

`, expectedConfig: Config{
				LoggerCfg: &zap.Config{
					Level:       info,
					Development: false,
				},
				MetricsConfig: &MetricsConfig{
					BackendDestination:     "prometheus",
					PrometheusPort:         9092,
					ReportingPeriodSeconds: 5,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, err := Parse([]byte(tc.content))
			require.Equal(t, err, nil)

			t.Logf("Config: %+v", c)
			t.Logf("Config: %+v", c.MetricsConfig)
			t.Logf("Config: %+v", c.ZapLoggerConfig)

			// Compare logger configuration elements.
			require.Equal(t, tc.expectedConfig.LoggerCfg.Level, c.LoggerCfg.Level)
			require.Equal(t, tc.expectedConfig.LoggerCfg.Development, c.LoggerCfg.Development)

			// Compare metric configuration.
			require.Equal(t, tc.expectedConfig.MetricsConfig, c.MetricsConfig)

		})
	}
}

func TestConfigToMap(t *testing.T) {
	info := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	cases := map[string]struct {
		config      Config
		expectedMap map[string]string
	}{
		"simple": {
			config: Config{
				LoggerCfg: &zap.Config{
					Level:       info,
					Development: false,
				},
				MetricsConfig: &MetricsConfig{
					BackendDestination:     "prometheus",
					PrometheusPort:         9092,
					ReportingPeriodSeconds: 5,
				},
			},
			expectedMap: map[string]string{
				"metrics.backend-destination":      "prometheus",
				"metrics.prometheus-port":          "9092",
				"metrics.reporting-period-seconds": "5",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m, err := tc.config.ToMap()
			require.NoError(t, err, "config to map failed")
			require.Equal(t, tc.expectedMap, m)
		})
	}
}
