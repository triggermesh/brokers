package observability

import (
	"encoding/json"
	"fmt"
	"os"

	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

const zapLoggerElement = "zap-logger-config"

type Config struct {
	LoggerCfg *zap.Config
	// TODO metrics
}

func ReadFromFile(file string) (*Config, error) {
	f, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %+w", file, err)
	}

	return Parse(f)
}

func zapConfigFromJSON(configJSON string) (*zap.Config, error) {
	loggingCfg := defaultZapConfig()

	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), loggingCfg); err != nil {
			return nil, err
		}
	}
	return loggingCfg, nil
}

func defaultZapConfig() *zap.Config {
	lc := zap.NewProductionConfig()
	return &lc
}

func Parse(content []byte) (*Config, error) {
	cfg := map[string]string{}
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("could not parse observability data into string map: %+w", err)
	}

	return ParseFromMap(cfg)
}

func ParseFromMap(content map[string]string) (*Config, error) {
	zc, err := zapConfigFromJSON(content[zapLoggerElement])
	if err != nil {
		return nil, fmt.Errorf("could not parse zap logger config: %+w", err)
	}

	return &Config{
		LoggerCfg: zc,
	}, nil
}

func DefaultConfig() *Config {
	return &Config{
		LoggerCfg: defaultZapConfig(),
	}
}
