.PHONY: migrate migrate-down run build test lint generate

DB_URL ?= $(shell grep '^DATABASE_URL=' .env | cut -d= -f2-)

migrate:
	GOSUMDB=sum.golang.org go tool goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	GOSUMDB=sum.golang.org go tool goose -dir migrations postgres "$(DB_URL)" down

run:
	GOSUMDB=sum.golang.org go run ./cmd/trip-service

build:
	GOSUMDB=sum.golang.org go build -o bin/trip-service ./cmd/trip-service

test:
	GOSUMDB=sum.golang.org go test -race ./...

lint:
	golangci-lint run

generate:
	GOSUMDB=sum.golang.org go tool oapi-codegen -generate types,chi-server -package api -include-operation-ids createTrip,getTrip,finishTrip,health,ready -o api/api.gen.go contracts/openapi/trip-service.openapi.yaml
