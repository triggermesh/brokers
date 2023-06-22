// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	brokerConfigPath = "/broker/config/path"

	brokerConfigNamespace  = "my-namespace"
	brokerConfigSecretName = "broker-secret"
	brokerConfigSecretKey  = "broker-secret-key"
)

func TestGlobalsValidation(t *testing.T) {
	testCases := map[string]struct {
		globals Globals

		expectedErr          string
		expectedConfigMethod ConfigMethod
	}{
		"file watcher": {
			globals: Globals{
				BrokerConfigPath: brokerConfigPath,
			},
			expectedConfigMethod: ConfigMethodFileWatcher,
		},
		"file polling": {
			globals: Globals{
				BrokerConfigPath:    brokerConfigPath,
				ConfigPollingPeriod: "PT1S",
			},
			expectedConfigMethod: ConfigMethodFilePoller,
		},
		"kubernetes secret": {
			globals: Globals{
				KubernetesNamespace:              brokerConfigNamespace,
				KubernetesBrokerConfigSecretName: brokerConfigSecretName,
				KubernetesBrokerConfigSecretKey:  brokerConfigSecretKey,
			},
			expectedConfigMethod: ConfigMethodKubernetesSecretMapWatcher,
		},
		"inline configuration": {
			globals: Globals{
				BrokerConfig: `{"yada":"yada"}`,
			},
			expectedConfigMethod: ConfigMethodInline,
		},
		"inline configuration with default broker config path": {
			globals: Globals{
				BrokerConfigPath: defaultBrokerConfigPath,
				BrokerConfig:     `{"yada":"yada"}`,
			},
			expectedConfigMethod: ConfigMethodInline,
		},
		"not valid polling": {
			globals: Globals{
				BrokerConfigPath:    brokerConfigPath,
				ConfigPollingPeriod: "1312",
			},
			expectedErr:          "polling period is not an ISO8601 duration",
			expectedConfigMethod: ConfigMethodUnknown,
		},
		"mixed watcher and kubernetes": {
			globals: Globals{
				BrokerConfigPath:                 brokerConfigPath,
				KubernetesNamespace:              brokerConfigNamespace,
				KubernetesBrokerConfigSecretName: brokerConfigSecretName,
				KubernetesBrokerConfigSecretKey:  brokerConfigSecretKey,
			},
			expectedErr:          "Cannot use Broker file for configuration when a Kubernetes Secret is used for the broker.",
			expectedConfigMethod: ConfigMethodUnknown,
		},
		"mixed poller and environment": {
			globals: Globals{
				BrokerConfigPath:    brokerConfigPath,
				ConfigPollingPeriod: "PT1S",
				BrokerConfig:        `{"yada":"yada"}`,
			},
			expectedErr:          "Inline config cannot be used along with local file configuration.",
			expectedConfigMethod: ConfigMethodUnknown,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			err := tc.globals.Validate()

			if err != nil || tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			}
			assert.Equal(t, tc.expectedConfigMethod, tc.globals.ConfigMethod, "ConfigMethod does not match expected.")
		})
	}
}
