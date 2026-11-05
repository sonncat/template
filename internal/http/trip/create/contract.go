//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create

package create

import (
	"context"

	"github.com/sonncat/template/internal/model"
)

type tripUseCase interface {
	Execute(ctx context.Context, trip *model.Trip) (*model.Trip, error)
}
