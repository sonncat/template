//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=save

package save

import (
	"context"

	"github.com/sonncat/template/internal/model"
)

type positionRepository interface {
	SavePosition(ctx context.Context, position *model.Position) error
}

type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}
