package controller

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	crctrl "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type Manager struct {
	manager manager.Manager
	rs      *reconcileSecret

	logger *zap.SugaredLogger
}

func NewManager(namespace string, logger *zap.SugaredLogger) (*Manager, error) {
	// There is no easy way of bridging the gap between Knative style
	// and controller-runtime style loggers.
	crlog.SetLogger(crzap.New())

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Namespace:          namespace,
		MetricsBindAddress: "0",
	})
	if err != nil {
		return nil, fmt.Errorf("unable to set up controller manager: %w", err)
	}

	return &Manager{
		manager: mgr,
		logger:  logger,
	}, nil
}

func (m *Manager) AddSecretController(name, key string) error {
	m.logger.Info("Setting up secrets controller")
	m.rs = &reconcileSecret{
		name:   name,
		key:    key,
		client: m.manager.GetClient(),
		logger: m.logger,
	}

	c, err := crctrl.New("secret-controller", m.manager, crctrl.Options{
		Reconciler: m.rs,
	})

	if err != nil {
		return fmt.Errorf("unable to set up secret controller: %w", err)
	}
	if err := c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestForObject{},
		predicate.NewPredicateFuncs(func(o client.Object) bool { return o.GetName() == name }),
	); err != nil {
		return fmt.Errorf("unable to set watch for secrets: %w", err)
	}

	return nil
}

func (m *Manager) AddSecretCallback(cb SecretConfigCallback) {
	m.rs.cbs = append(m.rs.cbs, cb)
}

func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting manager")
	if err := m.manager.Start(ctx); err != nil {
		return fmt.Errorf("unable to run controller manager: %w", err)
	}

	return nil
}
