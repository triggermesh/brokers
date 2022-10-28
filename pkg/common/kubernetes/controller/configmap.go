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

	"github.com/triggermesh/brokers/pkg/config/observability"
)

type ConfigMapObservabilityCallback func(cfg *observability.Config)

// reconcileObservabilityConfigMap reconciles the observability ConfigMap.
type reconcileObservabilityConfigMap struct {
	name string
	cbs  []ConfigMapObservabilityCallback

	client client.Client
	logger *zap.SugaredLogger
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &reconcileObservabilityConfigMap{}

func (r *reconcileObservabilityConfigMap) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(ctx, request.NamespacedName, cm)
	if errors.IsNotFound(err) {
		r.logger.Errorw("could not find ConfigMap", zap.String("name", request.NamespacedName.String()))
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not fetch ConfigMap: %w", err)
	}

	r.logger.Infow("Reconciling ConfigMap", zap.String("name", cm.Name))
	cfg, err := observability.ParseFromMap(cm.Data)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error parsing observability config from ConfigMap %q: %w", cm.Name, err)
	}

	for _, cb := range r.cbs {
		cb(cfg)
	}

	return reconcile.Result{}, nil
}
