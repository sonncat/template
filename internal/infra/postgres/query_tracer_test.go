package db

import "testing"

func TestQueryCollection(t *testing.T) {
	const tripsTable = "trips"

	tests := []struct {
		name string
		sql  string
		want string
	}{
		{name: "select", sql: "SELECT id FROM trips WHERE id = $1", want: tripsTable},
		{name: "insert trip", sql: "INSERT INTO trips (id) VALUES ($1)", want: tripsTable},
		{
			name: "insert history",
			sql:  "INSERT INTO trip_status_history (trip_id) VALUES ($1)",
			want: "trip_status_history",
		},
		{name: "update", sql: "UPDATE trips SET status = $1", want: tripsTable},
		{name: "unknown table", sql: "SELECT * FROM other_table", want: ""},
		{name: "no table", sql: "SELECT 1", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := queryCollection(tt.sql); got != tt.want {
				t.Errorf("queryCollection(%q) = %q, want %q", tt.sql, got, tt.want)
			}
		})
	}
}
