package status

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/triggermesh/brokers/pkg/status"
)

type kubernetesBackend struct {
	// Instance must be unique for every instance of the broker, it will
	// be used as the root element for the status reporting structure.
	instance string

	// ConfigMap object and key identification
	key   client.ObjectKey
	cmkey string

	client client.Client
	logger *zap.SugaredLogger
}

// Returns a kubernetes status manager object. Parameters are:
// - name, namespace and key for the ConfigMap where the status will be written to.
// - identifier for this broker instance.
// - cache expiration that will force a new status write operation
// - resync period that check for pending changes and writes to the ConfigMap if any.
// - kubernetes client
// - logger
func NewKubernetesBackend(name, namespace, cmkey, instance string, kc client.Client, log *zap.SugaredLogger) status.Backend {
	km := &kubernetesBackend{
		instance: instance,
		key: client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		},

		cmkey:  cmkey,
		client: kc,

		logger: log,
	}

	return km
}

func (b *kubernetesBackend) UpdateStatus(ctx context.Context, s *status.Status) error {
	// Read current contents of the status at the ConfigMap.
	cm := &corev1.ConfigMap{}
	err := b.client.Get(ctx, b.key, cm)
	if err != nil {
		return fmt.Errorf("could not read status configmap: %w", err)
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	// Parse ConfigMap key contents into a Status structure.
	// If it does not exists or is not formatted an empty one will be used.
	st := map[string]status.Status{}
	data, ok := cm.Data[b.cmkey]
	if ok {
		if err := json.Unmarshal([]byte(data), &st); err != nil {
			b.logger.Errorw("status ConfigMap contents could not be parsed. Status will be overwritten", zap.Error(err))
		}
	}

	st[b.instance] = *s
	bst, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	cm.Data[b.cmkey] = string(bst)
	if err = b.client.Update(ctx, cm, &client.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}
