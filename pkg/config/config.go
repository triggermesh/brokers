// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"gopkg.in/yaml.v3"
)

func Parse(config string) (*Config, error) {
	c := &Config{}
	err := yaml.Unmarshal([]byte(config), c)

	return c, err
}
