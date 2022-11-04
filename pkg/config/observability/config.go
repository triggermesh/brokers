package observability

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

const (
	zapLoggerConfigLabel        = "zap-logger-config"
	backendDestinationLabel     = "metrics.backend-destination"
	reportingPeriodSecondsLabel = "metrics.reporting-period-seconds"
	prometheusPortLabel         = "metrics.prometheus-port"
	openCensusAddressLabel      = "metrics.opencensus-address"
)

type Config struct {
	*MetricsConfig  `json:",inline"`
	ZapLoggerConfig string `json:"zap-logger-config"`

	LoggerCfg *zap.Config `json:"-"`
}

func (c *Config) ToMap() (map[string]string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	mi := map[string]interface{}{}
	err = json.Unmarshal(b, &mi)
	if err != nil {
		return nil, err
	}

	m := make(map[string]string, len(mi))
	for k, v := range mi {
		if s, ok := v.(string); ok {
			if s != "" {
				m[k] = s
			}
			continue
		}
		if f, ok := v.(float64); ok {
			if f != 0 {
				m[k] = strconv.Itoa(int(f))
			}
			continue
		}
		return nil, fmt.Errorf("config element %s type is unexpected: %T(%v)", k, v, v)
	}

	return m, nil
}

type MetricsConfig struct {
	BackendDestination     string `json:"metrics.backend-destination"`
	ReportingPeriodSeconds int    `json:"metrics.reporting-period-seconds"`
	PrometheusPort         int    `json:"metrics.prometheus-port"`
	OpenCensusAddress      string `json:"metrics.opencensus-address"`
}

func ReadFromFile(file string) (*Config, error) {
	f, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %+w", file, err)
	}

	return Parse(f)
}

func defaultZapConfig() *zap.Config {
	lc := zap.NewProductionConfig()
	return &lc
}

func DefaultConfig() *Config {
	return &Config{
		LoggerCfg: defaultZapConfig(),
	}
}

func Parse(content []byte) (*Config, error) {
	cfg := &Config{}
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse observability data into string map: %+w", err)
	}

	loggingCfg := defaultZapConfig()
	if cfg.ZapLoggerConfig != "" {
		if err := json.Unmarshal([]byte(cfg.ZapLoggerConfig), loggingCfg); err != nil {
			return nil, err
		}
	}

	cfg.LoggerCfg = loggingCfg

	return cfg, nil
}

func ParseFromMap(content map[string]string) (*Config, error) {
	cfg := &Config{
		LoggerCfg: defaultZapConfig(),
	}

	if c, ok := content[zapLoggerConfigLabel]; ok {
		if err := json.Unmarshal([]byte(c), cfg.LoggerCfg); err != nil {
			return nil, fmt.Errorf("could not unmarshal zap logger config: %w", err)
		}
	}

	if c, ok := content[backendDestinationLabel]; ok {
		cfg.BackendDestination = c
	}

	if c, ok := content[reportingPeriodSecondsLabel]; ok {
		p, err := strconv.Atoi(c)
		if err != nil {
			return nil, fmt.Errorf("reporting period seconds must be an integer number: %w", err)
		}
		cfg.ReportingPeriodSeconds = p
	}

	if c, ok := content[prometheusPortLabel]; ok {
		p, err := strconv.Atoi(c)
		if err != nil {
			return nil, fmt.Errorf("prometheus port must be an integer number: %w", err)
		}
		cfg.PrometheusPort = p
	}

	if c, ok := content[openCensusAddressLabel]; ok {
		cfg.OpenCensusAddress = c
	}

	return cfg, nil
}
