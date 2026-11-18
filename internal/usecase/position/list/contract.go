//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=list

package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

type positionRepository interface {
	ListPositions(ctx context.Context, tripID uuid.UUID) ([]model.Position, error)
}
