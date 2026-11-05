package domain

import (
	"errors"

	"github.com/sonncat/template/internal/domain/trip"
)

// Имена сохранены для кода, который ещё использует пакет domain
var (
	ErrTripNotFound  = trip.ErrTripNotFound
	ErrTripCompleted = trip.ErrTripCompleted
	ErrDriverBusy    = trip.ErrDriverBusy

	ErrInvalidRequest = errors.New("invalid request")
)
