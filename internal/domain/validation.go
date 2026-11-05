package domain

import "github.com/sonncat/template/internal/domain/validation"

// ValidationError — общее имя ошибки валидации для остальных слоёв
type ValidationError = validation.Error
