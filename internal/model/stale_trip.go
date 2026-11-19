package model

import "github.com/google/uuid"

// StaleTrip содержит данные, необходимые для запроса координаты
type StaleTrip struct {
	TripID   uuid.UUID
	DriverID uuid.UUID
}
