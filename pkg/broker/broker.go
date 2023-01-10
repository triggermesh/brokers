// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package broker

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/triggermesh/brokers/pkg/backend"
	"github.com/triggermesh/brokers/pkg/broker/cmd"
	"github.com/triggermesh/brokers/pkg/common/fs"
	"github.com/triggermesh/brokers/pkg/common/kubernetes/controller"
	cfgbroker "github.com/triggermesh/brokers/pkg/config/broker"
	cfgbpoller "github.com/triggermesh/brokers/pkg/config/broker/poller"
	cfgbwatcher "github.com/triggermesh/brokers/pkg/config/broker/watcher"
	cfgopoller "github.com/triggermesh/brokers/pkg/config/observability/poller"
	cfgowatcher "github.com/triggermesh/brokers/pkg/config/observability/watcher"
	"github.com/triggermesh/brokers/pkg/ingest"
	"github.com/triggermesh/brokers/pkg/ingest/metrics"
	"github.com/triggermesh/brokers/pkg/subscriptions"
)

type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopping Status = "stopping"
)

type Instance struct {
	backend      backend.Interface
	ingest       *ingest.Instance
	subscription *subscriptions.Manager
	bcw          *cfgbwatcher.Watcher
	ocw          *cfgowatcher.Watcher
	bcp          *cfgbpoller.Poller
	ocp          *cfgopoller.Poller
	km           *controller.Manager
	staticConfig *cfgbroker.Config
	status       Status

	logger *zap.SugaredLogger
}

func NewInstance(globals *cmd.Globals, b backend.Interface) (*Instance, error) {
	globals.Logger.Debug("Creating subscription manager")

	// Create subscription manager.
	sm, err := subscriptions.New(globals.Context, globals.Logger.Named("subs"), b)
	if err != nil {
		return nil, err
	}

	globals.Logger.Debug("Creating HTTP ingest server")
	// Create metrics reporter.
	ir, err := metrics.NewReporter(globals.Context)
	if err != nil {
		return nil, err
	}

	i := ingest.NewInstance(ir, globals.Logger.Named("ingest"),
		ingest.InstanceWithPort(globals.Port),
	)

	globals.Logger.Debug("Creating broker instance")
	broker := &Instance{
		backend:      b,
		ingest:       i,
		subscription: sm,
		status:       StatusStopped,

		logger: globals.Logger.Named("broker"),
	}

	switch globals.ConfigMethod {

	case cmd.ConfigMethodFileWatcher:
		// The ConfigWatcher will read the configfile and call registered
		// callbacks upon start and everytime the configuration file
		// is updated.
		cfw, err := fs.NewCachedFileWatcher(globals.Logger.Named("fswatch"))
		if err != nil {
			return nil, err
		}

		configPath, err := filepath.Abs(globals.BrokerConfigPath)
		if err != nil {
			return nil, fmt.Errorf("error resolving to absolute path %q: %w", globals.BrokerConfigPath, err)
		}

		globals.Logger.Debugw("Creating watcher for broker configuration", zap.String("file", configPath))
		bcfgw, err := cfgbwatcher.NewWatcher(cfw, configPath, globals.Logger.Named("cgfwatch"))
		if err != nil {
			return nil, fmt.Errorf("error adding broker watcher for %q: %w", configPath, err)
		}

		broker.bcw = bcfgw

		if globals.ObservabilityConfigPath != "" {
			var ocfgw *cfgowatcher.Watcher
			if globals.ObservabilityConfigPath != "" {
				obsCfgPath, err := filepath.Abs(globals.ObservabilityConfigPath)
				if err != nil {
					return nil, fmt.Errorf("error resolving to absolute path %q: %w", globals.ObservabilityConfigPath, err)
				}

				globals.Logger.Debugw("Creating watcher for observability configuration", zap.String("file", obsCfgPath))
				ocfgw, err = cfgowatcher.NewWatcher(cfw, obsCfgPath, globals.Logger.Named("ocgfwatch"))
				if err != nil {
					return nil, fmt.Errorf("error adding observability watcher for %q: %w", globals.ObservabilityConfigPath, err)
				}

				ocfgw.AddCallback(globals.UpdateLogLevel)
				ocfgw.AddCallback(globals.UpdateMetricsOptions)
				broker.ocw = ocfgw
			}
		}

	case cmd.ConfigMethodKubernetesSecretMapWatcher:
		km, err := controller.NewManager(globals.KubernetesNamespace, globals.Logger.Named("controller"))
		if err != nil {
			return nil, fmt.Errorf("error creating kubernetes controller manager: %w", err)
		}

		if err = km.AddSecretControllerForBrokerConfig(
			globals.KubernetesBrokerConfigSecretName,
			globals.KubernetesBrokerConfigSecretKey); err != nil {
			return nil, fmt.Errorf("error adding broker Secret reconciler to controller: %w", err)
		}

		km.AddSecretCallbackForBrokerConfig(i.UpdateFromConfig)
		km.AddSecretCallbackForBrokerConfig(sm.UpdateFromConfig)

		if globals.KubernetesObservabilityConfigMapName != "" {
			if err = km.AddConfigMapControllerForObservability(globals.KubernetesObservabilityConfigMapName); err != nil {
				return nil, fmt.Errorf("error adding observability ConfigMap reconciler to controller: %w", err)
			}
			km.AddConfigMapCallbackForObservabilityConfig(globals.UpdateLogLevel)
			km.AddConfigMapCallbackForObservabilityConfig(globals.UpdateMetricsOptions)
		}

		broker.km = km

	case cmd.ConfigMethodFilePoller:
		p, err := fs.NewPoller(globals.PollingPeriod, globals.Logger.Named("poller"))
		if err != nil {
			return nil, fmt.Errorf("error creating file poller: %w", err)
		}

		configPath, err := filepath.Abs(globals.BrokerConfigPath)
		if err != nil {
			return nil, fmt.Errorf("error resolving to absolute path %q: %w", globals.BrokerConfigPath, err)
		}

		globals.Logger.Debugw("Creating poller for broker configuration", zap.String("file", configPath))
		bcfgp, err := cfgbpoller.NewPoller(p, configPath, globals.Logger.Named("cfgpoller"))
		if err != nil {
			return nil, fmt.Errorf("error adding broker poller for %q: %w", configPath, err)
		}

		broker.bcp = bcfgp

		if globals.ObservabilityConfigPath != "" {
			obsCfgPath, err := filepath.Abs(globals.ObservabilityConfigPath)
			if err != nil {
				return nil, fmt.Errorf("error resolving to absolute path %q: %w", globals.ObservabilityConfigPath, err)
			}

			globals.Logger.Debugw("Creating poller for observability configuration", zap.String("file", obsCfgPath))
			ocfgp, err := cfgopoller.NewPoller(p, obsCfgPath, globals.Logger.Named("ocfgpoller"))
			if err != nil {
				return nil, fmt.Errorf("error adding observability poller for %q: %w", obsCfgPath, err)
			}

			ocfgp.AddCallback(globals.UpdateLogLevel)
			ocfgp.AddCallback(globals.UpdateMetricsOptions)

			broker.ocp = ocfgp
		}

	case cmd.ConfigMethodInline:
		// Observability options were already set at globals initialize. We
		// only need to set ingest and subscription config here.

		cfg, err := cfgbroker.Parse(globals.BrokerConfig)
		if err != nil {
			return nil, fmt.Errorf("error parsing inline broker configuration: %w", err)
		}
		broker.staticConfig = cfg
	}

	return broker, nil
}

func (i *Instance) Start(inctx context.Context) error {
	i.logger.Debug("Starting broker instance")
	i.status = StatusStarting
	i.ingest.RegisterProbeHandler(i.ProbeHandler)

	sigctx, stop := signal.NotifyContext(inctx, os.Interrupt, syscall.SIGTERM)
	defer func() {
		stop()
		i.status = StatusStopped
	}()

	grp, ctx := errgroup.WithContext(sigctx)
	go func() {
		<-ctx.Done()
		// In case we receive the context done signal but the
		// status was already set to Stopped.
		if i.status != StatusStopped {
			i.status = StatusStopping
		}
	}()

	// Initialization will create structures, execute migrations
	// and claim non processed messages from the backend.
	i.logger.Debug("Initializing backend")
	err := i.backend.Init(ctx)
	if err != nil {
		return fmt.Errorf("could not initialize backend: %w", err)
	}

	// Start is a blocking function that will read messages from the backend
	// implementation and send them to the subscription manager dispatcher.
	// When the dispatcher returns the message is marked as processed.
	i.logger.Debug("Starting backend routine")
	grp.Go(func() error {
		return i.backend.Start(ctx)
	})

	// Setup broker config file watchers only if configured.
	if i.bcw != nil {
		// ConfigWatcher will callback reconfigurations for:
		// - Ingest: if authentication parameters are updated.
		// - Subscription manager: if triggers configurations changes.
		i.logger.Debug("Adding config watcher callbacks")
		i.bcw.AddCallback(i.ingest.UpdateFromConfig)
		i.bcw.AddCallback(i.subscription.UpdateFromConfig)

		// Start the configuration watcher for brokers.
		// There is no need to add it to the wait group
		// since it cleanly exits when context is done.
		i.logger.Debug("Starting broker configuration watcher")
		if err = i.bcw.Start(ctx); err != nil {
			return fmt.Errorf("could not start broker configuration watcher: %w", err)
		}
	}

	// Start observability config file watchers only if configured.
	if i.ocw != nil {
		i.logger.Debug("Starting observability configuration watcher")
		if err = i.ocw.Start(ctx); err != nil {
			return fmt.Errorf("could not start observability configuration watcher: %w", err)
		}
	}

	if i.bcp != nil {
		i.logger.Debug("Adding config poller callbacks")
		i.bcp.AddCallback(i.ingest.UpdateFromConfig)
		i.bcp.AddCallback(i.subscription.UpdateFromConfig)

		// Start the configuration poller for brokers.
		// There is no need to add it to the wait group
		// since it cleanly exits when context is done.
		i.logger.Debug("Starting broker configuration poller")
		if err = i.bcp.Start(ctx); err != nil {
			return fmt.Errorf("could not start broker configuration poller: %w", err)
		}

	}

	// Start observability config file pollers only if configured.
	if i.ocp != nil {
		i.logger.Debug("Starting observability configuration poller")
		if err = i.ocp.Start(ctx); err != nil {
			return fmt.Errorf("could not start observability configuration poller: %w", err)
		}
	}

	// Start controller only if kubernetes informers are configured.
	if i.km != nil {
		grp.Go(func() error {
			err := i.km.Start(ctx)
			return err
		})
	}

	// Static config is configured once when starting.
	if i.staticConfig != nil {
		i.ingest.UpdateFromConfig(i.staticConfig)
		i.subscription.UpdateFromConfig(i.staticConfig)
	}

	// Register producer function for received events at ingest.
	i.ingest.RegisterCloudEventHandler(i.backend.Produce)

	// TODO register probes at ingest

	// Start the server that ingests CloudEvents.
	grp.Go(func() error {
		err := i.ingest.Start(ctx)
		return err
	})

	i.status = StatusRunning

	return grp.Wait()
}

func (i *Instance) GetStatus() Status {
	return i.status
}

func (i *Instance) ProbeHandler() error {
	// TODO check each service
	return nil
}
