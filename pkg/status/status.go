package status

type Manager interface {
	UpdateIngestStatus(is *IngestStatus)
	// UpdateStatus(ctx context.Context, s *Status) error
	//UpdateSubscription()
	//DeleteSubscription()
}
