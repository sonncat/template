//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=get

package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

type tripUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}
