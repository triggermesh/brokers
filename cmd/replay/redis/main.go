// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	replay "github.com/triggermesh/brokers/pkg/replay/redis"
	pkgadapter "knative.dev/eventing/pkg/adapter/v2"
)

func main() {
	pkgadapter.Main("replay-adapter", replay.EnvAccessorCtor, replay.NewAdapter)
}
