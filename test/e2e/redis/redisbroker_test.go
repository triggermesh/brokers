//go:build e2e
// +build e2e

package redis

import (
	"context"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/triggermesh/brokers/cmd/redis-broker/cmd"
	pkgcmd "github.com/triggermesh/brokers/pkg/cmd"
	"github.com/triggermesh/brokers/pkg/config"
	"github.com/triggermesh/brokers/test/e2e/lib"
)

const (
	initializationWait = time.Second * 2
	exitTimeout        = time.Second * 5
)

func TestRedisBroker(t *testing.T) {
	finished := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	cfgfile, err := ioutil.TempFile("", "broker-*.conf")
	require.NoError(t, err, "Failed to create a temporary configuration file")
	defer os.Remove(cfgfile.Name())

	cfg := config.Config{
		Triggers: map[string]config.Trigger{
			"test": {
				Target: config.Target{
					URL: "http://localhost:9090",
				},
			},
		},
	}

	b, err := yaml.Marshal(cfg)
	require.NoError(t, err, "Failed to create configuration for broker")

	_, err = cfgfile.Write(b)
	require.NoError(t, err, "Failed to write configuration file for broker")

	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	observedLogger := zap.New(observedZapCore)
	globals := &pkgcmd.Globals{
		Logger:     observedLogger.Sugar(),
		Context:    ctx,
		ConfigPath: cfgfile.Name(),
	}

	rb := cmd.StartCmd{}

	go func() {
		wg.Add(1)
		finished <- rb.Run(globals)
		wg.Done()
	}()

	// Create consumer
	c, err := lib.NewConsumer(9090)
	assert.NoError(t, err, "Failed creating consumer")

	go func() {
		wg.Add(1)
		err := c.Start(ctx)
		wg.Done()
		require.NoError(t, err, "Failed consuming events")

	}()

	select {
	case <-time.After(initializationWait):
		// Launch tests
		p := lib.NewProducer("http://localhost:8080")
		err = p.Produce(ctx, lib.NewCloudEvent())
		assert.NoError(t, err, "Failed producing event")

	case err = <-finished:
		require.NoError(t, err, "Failed running broker server")
	}

	// Wait for condition based on logs
	for {
		// TODO add timeout
		// TODO add better condition
		ol := observedLogs.FilterMessageSnippet("Performing")
		if ol.Len() == 0 {
			time.Sleep(1 * time.Second)
			continue
		}
		break
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

	wg.Wait()
	for i, v := range c.Store.GetAll() {
		t.Logf("Item received %d: %v", i, v.Event.String())
	}

}
