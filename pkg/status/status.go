package status

import "context"

type Manager interface {
	UpdateStatus(ctx context.Context, s *Status) error
	//UpdateSubscription()
	//DeleteSubscription()
}
