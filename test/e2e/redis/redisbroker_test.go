//go:build e2e
// +build e2e

package redis

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/triggermesh/brokers/cmd/redis-broker/cmd"
	pkgcmd "github.com/triggermesh/brokers/pkg/cmd"
	"github.com/triggermesh/brokers/test/e2e/lib"
)

const (
	initializationWait = time.Second * 2
	exitTimeout        = time.Second * 5
)

func TestRedisBroker(t *testing.T) {
	finished := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())

	globals := &pkgcmd.Globals{
		Logger:  zaptest.NewLogger(t).Sugar(),
		Context: ctx,
	}

	rb := cmd.StartCmd{}

	go func() {
		finished <- rb.Run(globals)
	}()

	// Avoid calling cancel on context too soon.
	var err error
	select {
	case <-time.After(initializationWait):
		// Launch tests
		p := lib.NewProducer("http://localhost:8080")
		err := p.Produce(ctx, lib.NewCloudEvent())
		assert.NoError(t, err, "Failed producing event ")

	case err = <-finished:
		require.NoError(t, err, "Failed running broker server")
	}

	cancel()
	if err != nil {
		return
	}

	select {
	case err := <-finished:
		require.NoError(t, err, "Failed running broker server")
	case <-time.After(exitTimeout):
		assert.Fail(t, "Closing the Redis broker timed out")
	}
}
