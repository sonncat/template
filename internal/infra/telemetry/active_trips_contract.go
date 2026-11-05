//go:generate go tool mockgen -source=active_trips_contract.go -destination=active_trips_mocks_test.go -package=telemetry

package telemetry

import "context"

type activeTripsCounter interface {
	CountActive(ctx context.Context) (int64, error)
}
