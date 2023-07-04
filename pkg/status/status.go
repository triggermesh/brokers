package status

type Manager interface {
	UpdateIngestStatus(is *IngestStatus)
	EnsureSubscription(name string, ss *SubscriptionStatus)
	EnsureNoSubscription(name string)
}
