package status

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type kubernetesManager struct {
	instance string
	cached   *Status
	key      client.ObjectKey
	cmkey    string
	client   client.Client

	logger *zap.SugaredLogger
	m      sync.Mutex
}

func NewKubernetesManager(name, namespace, cmkey, instance string, kc client.Client, log *zap.SugaredLogger) Manager {
	return &kubernetesManager{
		instance: instance,
		cached:   &Status{},
		key: client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		},
		cmkey:  cmkey,
		client: kc,

		logger: log,
		m:      sync.Mutex{},
	}
}

func (m *kubernetesManager) UpdateSubscription() {
	// Compose new status
	// Read Configmap
	// UpdateConfigMap
}

func (m *kubernetesManager) DeleteSubscription() {

}

func (m *kubernetesManager) UpdateStatus(ctx context.Context, s *Status) error {
	m.m.Lock()
	defer m.m.Unlock()

	cm := &corev1.ConfigMap{}
	err := m.client.Get(ctx, m.key, cm)
	if err != nil {
		return err
	}

	st := map[string]Status{}
	data, ok := cm.Data[m.cmkey]
	if ok {
		if err = json.Unmarshal([]byte(data), &st); err != nil {
			m.logger.Errorw("status ConfigMap contents could not be parsed. Status will be overwritten", zap.Error(err))
		}
	}

	st[m.instance] = *s

	b, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	cm.Data[m.cmkey] = string(b)
	m.client.Update(ctx, cm, &client.UpdateOptions{})

	return nil
}
