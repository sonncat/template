package domain

import "testing"

func TestValidationError(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		message string
		want    string
	}{
		{
			name:    "with field",
			field:   "price",
			message: "must be greater than zero",
			want:    "price: must be greater than zero",
		},
		{
			name:    "without field",
			message: "invalid request",
			want:    "invalid request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ValidationError{
				Field:   tt.field,
				Message: tt.message,
			}

			if got := err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}
