package kubernetes

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// func initializeControllerLogger() func() {
// 	return func() {
// 		// There is no easy way of bridging the gap between Knative style
// 		// and controller-runtime style loggers.
// 		crlog.SetLogger(crzap.New())
// 	}
// }

// var once sync.Once

// func NewSecretInformer(ctx context.Context, logger *zap.SugaredLogger) error {
// 	once.Do(initializeControllerLogger())

// 	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
// 	if err != nil {
// 		return fmt.Errorf("unable to set up overall controller manager: %w", err)
// 	}

// 	logger.Info("Setting up secrets controller")
// 	c, err := controller.New("secret-controller", mgr, controller.Options{
// 		Reconciler: &reconcileSecret{
// 			client: mgr.GetClient(),
// 			logger: logger,
// 		},
// 	})
// 	if err != nil {
// 		return fmt.Errorf("unable to set up secret controller: %w", err)
// 	}

// 	if err := c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}); err != nil {
// 		return fmt.Errorf("unable to watch secrets: %w", err)
// 	}

// 	logger.Info("Starting manager")
// 	if err := mgr.Start(ctx); err != nil {
// 		return fmt.Errorf("unable to run controller manager: %w", err)
// 	}

// 	return nil
// }

// reconcileSecrets reconciles the secret.
type reconcileSecret struct {
	client client.Client
	logger *zap.SugaredLogger
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &reconcileSecret{}

func (r *reconcileSecret) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// set up a convenient log object so we don't have to type request over and over again
	log := log.FromContext(ctx)

	// Fetch the ReplicaSet from the cache
	rs := &corev1.Secret{}
	err := r.client.Get(ctx, request.NamespacedName, rs)
	if errors.IsNotFound(err) {
		log.Error(nil, "Could not find Secrets")
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not fetch Secret: %+v", err)
	}

	log.Info("Reconciling Secret", "secret name", rs.Name)

	// Set the label if it is missing
	if rs.Labels == nil {
		rs.Labels = map[string]string{}
	}
	if rs.Labels["hello"] == "world" {
		return reconcile.Result{}, nil
	}

	// Update the ReplicaSet
	rs.Labels["hello"] = "world"
	err = r.client.Update(ctx, rs)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not write ReplicaSet: %+v", err)
	}

	return reconcile.Result{}, nil
}
