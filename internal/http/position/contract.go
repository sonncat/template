//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=position

package position

import (
	"context"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

type saveUseCase interface {
	Execute(ctx context.Context, position *model.Position) (*model.Position, error)
}

type listUseCase interface {
	Execute(ctx context.Context, tripID uuid.UUID) ([]model.Position, error)
}
