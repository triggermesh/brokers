package controller

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cfgbroker "github.com/triggermesh/brokers/pkg/config/broker"
)

// reconcileSecret reconciles the Secret.
type reconcileBrokerConfigSecret struct {
	name string
	key  string
	cbs  []SecretBrokerConfigCallback

	client client.Client
	logger *zap.SugaredLogger
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &reconcileBrokerConfigSecret{}

func (r *reconcileBrokerConfigSecret) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	s := &corev1.Secret{}
	err := r.client.Get(ctx, request.NamespacedName, s)
	if errors.IsNotFound(err) {
		r.logger.Errorw("could not find Secret", zap.String("name", s.Name))
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not fetch Secret: %w", err)
	}

	r.logger.Infow("Reconciling Secret", zap.String("name", s.Name))
	content, ok := s.Data[r.key]
	if !ok {
		r.logger.Errorw("empty Secret", zap.String("name", s.Name))
		return reconcile.Result{}, nil
	}

	if len(content) == 0 {
		// Discard file events that do not inform content.
		r.logger.Debugw("Received secret with empty contents", zap.String("name", s.Name))
		return reconcile.Result{}, nil
	}

	cfg, err := cfgbroker.Parse(string(content))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error parsing config from secret %q: %w", s.Name, err)
	}

	for _, cb := range r.cbs {
		cb(cfg)
	}

	return reconcile.Result{}, nil
}
