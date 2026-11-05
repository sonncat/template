package http

import (
	"context"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

// getTripUseCase описывает получение поездки, необходимое HTTP-обработчику
type getTripUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}
