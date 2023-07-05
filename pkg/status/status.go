package status

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

type Manager interface {
	UpdateIngestStatus(is *IngestStatus)
	EnsureSubscription(name string, ss *SubscriptionStatus)
	EnsureNoSubscription(name string)
}
