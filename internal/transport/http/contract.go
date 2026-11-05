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

// createTripUseCase описывает создание поездки, необходимое HTTP-обработчику
type createTripUseCase interface {
	Execute(ctx context.Context, trip *model.Trip) (*model.Trip, error)
}
