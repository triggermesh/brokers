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
	// Instance must be unique for every instance of the broker, it will
	// be used as the root element for the status reporting structure.
	instance string

	// ConfigMap object and key identification
	key   client.ObjectKey
	cmkey string

	// Cached structure for the status that avoids trying
	// rewrites when there are no status changes.
	//
	// The cached structure will be considered stale after some
	// configurable duration.
	//
	// lastStatusWrite checkpoints the last time the ConfigMap was written, and will be
	// combined with cacheExpiration to calculate cache expiration
	cached          *status.Status
	cacheExpiration time.Duration
	lastStatusWrite time.Time

	// The kubernetes status manager will run reconciling cycles according to the
	// resyncPeriod duration. If the status cache has expired, the ConfigMap will be
	// re-written by this reconciling process.
	//
	// pendingWrite is a flag set when a status change must be written at the next
	// reconciliation, no matter if the cached status is stale or not.
	//
	// A reconcile cycle can be explicitly run using the chReconcile channel.
	resyncPeriod time.Duration
	pendingWrite bool
	chReconcile  chan struct{}

	client client.Client
	logger *zap.SugaredLogger
	m      sync.Mutex
}

// Returns a kubernetes status manager object. Parameters are:
// - name, namespace and key for the ConfigMap where the status will be written to.
// - identifier for this broker instance.
// - cache expiration that will force a new status write operation
// - resync period that check for pending changes and writes to the ConfigMap if any.
// - kubernetes client
// - logger
func NewKubernetesManager(ctx context.Context, name, namespace, cmkey, instance string, cacheExpiration, resyncPeriod time.Duration, kc client.Client, log *zap.SugaredLogger) status.Manager {
	km := &kubernetesManager{
		instance: instance,
		key: client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		},

		cached: &status.Status{
			Subscriptions: make(map[string]*status.SubscriptionStatus),
		},
		cacheExpiration: cacheExpiration,
		resyncPeriod:    resyncPeriod,

		pendingWrite: true,
		chReconcile:  make(chan struct{}),

		cmkey:  cmkey,
		client: kc,

		logger: log,
		m:      sync.Mutex{},
	}

	go km.start(ctx)

	return km
}

func (m *kubernetesManager) isCacheStale() bool {
	return m.lastStatusWrite.Add(m.cacheExpiration).Before(time.Now())
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

		// Skip if there are no pending writes and the
		// cache is not stale
		if !m.pendingWrite && !m.isCacheStale() {
			continue
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

	if m.cached.Ingest.EqualStatus(is) {
		// If status is equal do not enqueue an update.
		return
	}

	m.cached.Ingest = *is

	if m.cached.Ingest.EqualSoftStatus(is) {
		// This update is not a priority, overwrite the ingest element and
		// let a different status update (like the status cache expired)
		// to writte it to the ConfigMap
		return
	}

	// This update must be written asap. Mark the flag and send the signal
	m.pendingWrite = true

	m.maybeEnqueueReconcile()
}

func (m *kubernetesManager) EnsureSubscription(name string, ss *status.SubscriptionStatus) {
	m.m.Lock()
	defer m.m.Unlock()

	s, ok := m.cached.Subscriptions[name]

	// Fill not informed values with existing.
	ss.Merge(s)

	switch {
	case ok && s.EqualStatus(ss):
		// If status is equal do not enqueue an update.

	case ok && s.EqualSoftStatus(ss):
		m.cached.Subscriptions[name] = ss
		// This update is not a priority, overwrite the ingest element and
		// let a different status update (like the status cache expired)
		// to writte it to the ConfigMap

	default:
		// Either a new subscription or an update that needs
		// to be written asap

		m.cached.Subscriptions[name] = ss

		m.pendingWrite = true
		m.maybeEnqueueReconcile()
	}
}
func (m *kubernetesManager) EnsureNoSubscription(name string) {
	m.m.Lock()
	defer m.m.Unlock()

	if _, ok := m.cached.Subscriptions[name]; ok {
		delete(m.cached.Subscriptions, name)
		m.pendingWrite = true
		m.maybeEnqueueReconcile()
	}
}

func (m *kubernetesManager) cachedToKubernetesConfigMap(ctx context.Context) error {
	m.m.Lock()
	defer m.m.Unlock()

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
	m.pendingWrite = false

	return nil
}

func (m *kubernetesManager) readConfigMap(ctx context.Context) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	err := m.client.Get(ctx, m.key, cm)
	if err != nil {
		return nil, err
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
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

func (m *kubernetesManager) maybeEnqueueReconcile() {
	select {
	case m.chReconcile <- struct{}{}:
	// Try to send but if busy skip

	default:
		m.logger.Debugw("Skipping status reconciliation due to full queue")
	}
}
