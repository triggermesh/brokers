package status

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/triggermesh/brokers/pkg/status"
)

const ConfigMapKey = "status"

type kubernetesManager struct {
	instance string
	key      client.ObjectKey
	cmkey    string
	client   client.Client
	cached   *status.Status

	// lastStatusWrite will be set to the last time that the ConfigMap was writen.
	// staleCache is the configured duration that this broker instance is forced to
	// re-write the configmap, even if no changes are detected.
	staleCache      time.Duration
	lastStatusWrite time.Time

	// pendingWrite is a flag set when a ConfigMap write fails and we are due to
	// try again later.
	pendingWrite bool

	chReconcile  chan struct{}
	resyncPeriod time.Duration

	logger *zap.SugaredLogger
	m      sync.Mutex
}

func NewKubernetesManager(ctx context.Context, name, namespace, cmkey, instance string, staleCache, resyncPeriod time.Duration, kc client.Client, log *zap.SugaredLogger) status.Manager {
	km := &kubernetesManager{
		instance: instance,
		key: client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		},

		resyncPeriod: resyncPeriod,
		staleCache:   staleCache,
		pendingWrite: true,
		chReconcile:  make(chan struct{}),

		cmkey:  cmkey,
		client: kc,

		logger: log,
		m:      sync.Mutex{},
	}

	km.start(ctx)

	return km
}

func (m *kubernetesManager) isCacheStale() bool {
	return m.lastStatusWrite.Add(m.staleCache).After(time.Now())
}

func (m *kubernetesManager) start(ctx context.Context) {
	ticker := time.NewTicker(m.resyncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-m.chReconcile:
			// fall out of select block
		case <-ticker.C:
			// fall out of select block
		case <-ctx.Done():
			return
		}

		err := m.cachedToKubernetesConfigMap(ctx)
		if err != nil {
			m.logger.Errorw("could not read status configmap", zap.Error(err))
		}

	}
}

func (m *kubernetesManager) UpdateIngestStatus(is *status.IngestStatus) {
	m.m.Lock()
	defer m.m.Unlock()

	if !m.pendingWrite && !m.isCacheStale() && m.cached.Ingest.EqualSoftStatus(is) {
		return
	}

	// Update cached status ingest element. Even if we fail to update it now we want
	// some later try to have it
	m.cached.Ingest = *is

	// Send reconcile signal
	m.chReconcile <- struct{}{}

	// err := m.cachedToKubernetesConfigMap(ctx)
	// if err != nil {
	// 	m.logger.Errorw("could not read status configmap", zap.Error(err))
	// }

	// cm, err := m.readConfigMap(ctx)
	// if err != nil {
	// 	m.logger.Errorw("could not read status configmap", zap.Error(err))
	// 	return
	// }

	// b, err := json.Marshal(m.cached)
	// if err != nil {
	// 	m.logger.Errorw("could not serialize cached status into string", zap.Error(err))
	// 	return
	// }

	// update configmap at kubernetes
	// st := m.statusFromConfigMap(cm)
	// st[m.instance] = *m.cached
	// if err = m.instanceStatusToConfigMap(ctx, cm, st); err != nil {
	// 	// mark as pending!!
	// 	m.logger.Errorw("could not serialize cached status into string", zap.Error(err))
	// }

	// m.lastStatusWrite = time.Now()

	// status, ok := cm.Data[m.cmkey]
	// if !ok {
	// 	// status =
	// }

	// st := map[string]status.Status{}
	// data, ok := cm.Data[m.cmkey]
	// bexisting := []byte(data)
	// if ok {
	// 	if err = json.Unmarshal(bexisting, &st); err != nil {
	// 		m.logger.Errorw("status ConfigMap contents could not be parsed. Status will be overwritten", zap.Error(err))
	// 	}
	// }

	// // // Compare incoming ingest
	// // if m.cached.Ingest.EqualSoftStatus(is) {
	// // 	// no hard changes found, only continue if an update is pending because some
	// // 	// previous cycle was canceled, or
	// // }
	// // m.cached.Ingest = *is

	// m.m.Lock()
	// defer m.m.Unlock()
	// m.cached.Ingest = *is

	// // TODO choose soft or hard

	// if st.EqualSoftStatus(m.cached) {
	// 	m.logger.Debug("no need to update status after ingest state update")
	// 	return
	// }

	// if err != nil {
	// 	m.logger.Errorw("could not parse RFC3339 Nano timestamp", zap.Error(err))
	// 	return
	// }

	// t := time.Now()
	// m.cached.LastUpdated = &t

	// if err != nil {
	// 	m.logger.Errorw("could not read status configmap", zap.Error(err))
	// 	return
	// }

	// st := map[string]status.Status{}
	// data, ok := cm.Data[m.cmkey]
	// bexisting := []byte(data)
	// if ok {
	// 	if err = json.Unmarshal(bexisting, &st); err != nil {
	// 		m.logger.Errorw("status ConfigMap contents could not be parsed. Status will be overwritten", zap.Error(err))
	// 	}
	// }

}

func (m *kubernetesManager) cachedToKubernetesConfigMap(ctx context.Context) error {
	cm, err := m.readConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("could not read status configmap: %w", err)
	}

	st := m.statusFromConfigMap(cm)
	t := time.Now()
	m.cached.LastUpdated = &t
	st[m.instance] = *m.cached

	b, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	cm.Data[m.cmkey] = string(b)
	if err = m.client.Update(ctx, cm, &client.UpdateOptions{}); err != nil {
		return err
	}

	m.lastStatusWrite = time.Now()
	return nil
}

func (m *kubernetesManager) readConfigMap(ctx context.Context) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	err := m.client.Get(ctx, m.key, cm)
	if err != nil {
		return nil, err
	}

	return cm, nil
}

func (m *kubernetesManager) statusFromConfigMap(cm *corev1.ConfigMap) map[string]status.Status {
	st := map[string]status.Status{}
	data, ok := cm.Data[m.cmkey]
	if !ok {
		return st
	}

	if err := json.Unmarshal([]byte(data), &st); err != nil {
		m.logger.Errorw("status ConfigMap contents could not be parsed. Status will be overwritten", zap.Error(err))
	}

	return st
}

// func (m *kubernetesManager) UpdateStatus(ctx context.Context, s *status.Status) error {
// 	m.m.Lock()
// 	defer m.m.Unlock()

// 	cm := &corev1.ConfigMap{}
// 	err := m.client.Get(ctx, m.key, cm)
// 	if err != nil {
// 		return err
// 	}

// 	st := map[string]status.Status{}
// 	data, ok := cm.Data[m.cmkey]
// 	bexisting := []byte(data)
// 	if ok {
// 		if err = json.Unmarshal(bexisting, &st); err != nil {
// 			m.logger.Errorw("status ConfigMap contents could not be parsed. Status will be overwritten", zap.Error(err))
// 		}
// 	}

// 	// current, ok := st[m.instance]
// 	// if ok {
// 	// 	current.
// 	// }

// 	st[m.instance] = *s

// 	b, err := json.Marshal(st)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal status: %w", err)
// 	}

// 	cm.Data[m.cmkey] = string(b)
// 	m.client.Update(ctx, cm, &client.UpdateOptions{})

// 	return nil
// }

// func (m *kubernetesManager) instanceStatusToConfigMap(ctx context.Context, cm *corev1.ConfigMap, status map[string]status.Status) error {
// 	b, err := json.Marshal(status)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal status: %w", err)
// 	}

// 	cm.Data[m.cmkey] = string(b)
// 	return m.client.Update(ctx, cm, &client.UpdateOptions{})
// }
