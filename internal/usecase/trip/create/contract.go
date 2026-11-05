//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create

package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

// tripRepository описывает операции с поездками, необходимые для создания
type tripRepository interface {
	Create(ctx context.Context, trip *model.Trip) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

// txManager выполняет функцию внутри транзакции
type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}
