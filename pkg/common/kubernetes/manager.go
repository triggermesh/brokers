package kubernetes

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type Manager struct {
	manager manager.Manager

	logger *zap.SugaredLogger
}

func NewManager(logger *zap.SugaredLogger) (*Manager, error) {
	// There is no easy way of bridging the gap between Knative style
	// and controller-runtime style loggers.
	crlog.SetLogger(crzap.New())

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		return nil, fmt.Errorf("unable to set up controller manager: %w", err)
	}

	return &Manager{
		manager: mgr,
		logger:  logger,
	}, nil
}

func (m *Manager) AddSecretController() error {
	m.logger.Info("Setting up secrets controller")
	c, err := controller.New("secret-controller", m.manager, controller.Options{
		Reconciler: &reconcileSecret{
			client: m.manager.GetClient(),
			logger: m.logger,
		},
	})
	if err != nil {
		return fmt.Errorf("unable to set up secret controller: %w", err)
	}
	if err := c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return fmt.Errorf("unable to set watch for secrets: %w", err)
	}

	return nil
}

func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting manager")
	if err := m.manager.Start(ctx); err != nil {
		return fmt.Errorf("unable to run controller manager: %w", err)
	}

	return nil
}
