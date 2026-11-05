package http

import (
	"context"

	"github.com/google/uuid"

	"github.com/sonncat/template/internal/model"
)

type getTripUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

type createTripUseCase interface {
	Execute(ctx context.Context, trip *model.Trip) (*model.Trip, error)
}

type finishTripUseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}
