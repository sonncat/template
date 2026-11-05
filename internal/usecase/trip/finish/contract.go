//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=finish

package finish

import (
	"context"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

type tripRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	Finish(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}
