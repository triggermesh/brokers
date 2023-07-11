package status

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type SubscriptionStatusChoice string

const (
	// The subscription has been created and is able to process events.
	SubscriptionStatusReady SubscriptionStatusChoice = "Ready"
	// The subscription has started processing events.
	SubscriptionStatusRunning SubscriptionStatusChoice = "Running"
	// The subscription could not be created.
	SubscriptionStatusFailed SubscriptionStatusChoice = "Failed"
	// The subscription will not receive further events and can be deleted.
	SubscriptionStatusComplete SubscriptionStatusChoice = "Complete"
)

type IngestStatusChoice string

const (
	// The ingest has been created and is able to receive events.
	IngestStatusReady IngestStatusChoice = "Ready"
	// The ingest has started receiving events.
	IngestStatusRunning IngestStatusChoice = "Running"
	// The ingest has been closed.
	IngestStatusClosed IngestStatusChoice = "Closed"
)

type Backend interface {
	UpdateStatus(ctx context.Context, s *Status) error
}

type Manager interface {
	RegisterBackendStatusWriters(b Backend)
	Start(ctx context.Context)

	UpdateIngestStatus(is *IngestStatus)
	EnsureSubscription(name string, ss *SubscriptionStatus)
	EnsureNoSubscription(name string)
}

type manager struct {
	// Cached structure for the status that avoids trying
	// rewrites when there are no status changes.
	//
	// The cached structure will be considered stale after some
	// configurable duration.
	//
	// lastStatusWrite checkpoints the last time the ConfigMap was written, and will be
	// combined with cacheExpiration to calculate cache expiration
	cached          *Status
	cacheExpiration time.Duration
	lastStatusWrite time.Time

	// The status manager will run reconciling cycles according to the resyncPeriod duration.
	// If the status cache has expired, the backend's update will be triggered.
	//
	// writeAsap is a flag set when a status change must be written at the next
	// reconciliation, no matter if the cached status is stale or not.
	//
	// A reconcile cycle can be explicitly run using the chReconcile channel.
	resyncPeriod time.Duration
	writeAsap    bool
	chReconcile  chan struct{}

	// registered status reconciler channels
	statusBackends []Backend

	log *zap.SugaredLogger
	m   sync.Mutex
}

func NewManager(cacheExpiration time.Duration, resyncPeriod time.Duration, log *zap.SugaredLogger) Manager {
	return &manager{
		cached:          &Status{Subscriptions: make(map[string]*SubscriptionStatus)},
		cacheExpiration: cacheExpiration,

		resyncPeriod: resyncPeriod,
		writeAsap:    true,
		chReconcile:  make(chan struct{}),

		statusBackends: []Backend{},

		log: log,
		m:   sync.Mutex{},
	}
}

func (m *manager) Start(ctx context.Context) {
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
		if !m.writeAsap && m.lastStatusWrite.Add(m.cacheExpiration).After(time.Now()) {
			continue
		}

		m.updateStatus(ctx)
	}
}

func (m *manager) updateStatus(ctx context.Context) {
	m.m.Lock()
	defer m.m.Unlock()

	// Touch last updated before sending to all backends.
	m.cached.LastUpdated = time.Now()

	// Iterate all registered status backends and call then.
	failed := false
	for i := range m.statusBackends {
		if err := m.statusBackends[i].UpdateStatus(ctx, m.cached); err != nil {
			m.log.Errorw("Failed updating the status", zap.Error(err))
			if failed {
				failed = true
			}
		}
	}

	if failed {
		// If the status update failed, raise the flag to force
		// write at the next cycle.
		m.writeAsap = true
	} else {
		// If all backends succeeded unset the writeAsap flag
		// and set the last status timestamp
		m.writeAsap = false
		m.lastStatusWrite = time.Now()
	}
}

func (m *manager) RegisterBackendStatusWriters(b Backend) {
	m.m.Lock()
	defer m.m.Unlock()

	m.statusBackends = append(m.statusBackends, b)
}

func (m *manager) UpdateIngestStatus(is *IngestStatus) {
	m.m.Lock()
	defer m.m.Unlock()

	if m.cached.Ingest.EqualStatus(is) {
		// If status is equal do not enqueue an update.
		return
	}

	// If status differs from existing, update it at the structure.
	m.cached.Ingest = *is

	// This update is not a priority, overwrite the ingest element and
	// let a different status update (like the status cache expired)
	// to writte it to the ConfigMap
	if m.cached.Ingest.EqualSoftStatus(is) {
		return
	}

	// This update must be written asap. Mark the flag and send the signal
	m.writeAsap = true

	m.maybeEnqueueReconcile()
}

func (m *manager) EnsureSubscription(name string, ss *SubscriptionStatus) {
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
		// to update the status.

	default:
		// Either a new subscription or an update that needs
		// to be written asap

		m.cached.Subscriptions[name] = ss

		m.writeAsap = true
		m.maybeEnqueueReconcile()
	}
}

func (m *manager) EnsureNoSubscription(name string) {
	m.m.Lock()
	defer m.m.Unlock()

	if _, ok := m.cached.Subscriptions[name]; ok {
		delete(m.cached.Subscriptions, name)
		m.writeAsap = true
		m.maybeEnqueueReconcile()
	}
}

func (m *manager) maybeEnqueueReconcile() {
	select {
	case m.chReconcile <- struct{}{}:
	// Try to send but if busy skip

	default:
		m.log.Debugw("Skipping status reconciliation due to full queue")
	}
}
