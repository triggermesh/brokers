package observability

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

func ReadFromFile(file string) (*Config, error) {
	f, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %+w", file, err)
	}

	return Parse(f)
}

func Parse(content []byte) (*Config, error) {
	c := &Config{}
	if err := yaml.Unmarshal([]byte(content), c); err != nil {
		return nil, fmt.Errorf("could not parse observability data into configuration: %+w", err)
	}

	return c, nil
}

func DefaultConfig() *Config {
	lc := zap.NewProductionConfig()
	cfg := &Config{
		Logging: &lc,
	}
	return cfg
}
