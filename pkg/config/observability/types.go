// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"go.uber.org/zap"
)

type Config struct {
	Logging *zap.Config `json:"logging,omitempty"`
	// TODO metrics
}
