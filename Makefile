.PHONY: generate migrate migrate-down migrate-status run build test

generate:
	GOSUMDB=sum.golang.org go tool oapi-codegen -generate types,chi-server -package api -include-operation-ids createTrip,getTrip,finishTrip,health,ready -o api/api.gen.go contracts/openapi/trip-service.openapi.yaml

migrate:
	set -a; . ./.env; set +a; GOSUMDB=sum.golang.org go tool goose -dir migrations postgres "$${DATABASE_URL}" up

migrate-down:
	set -a; . ./.env; set +a; GOSUMDB=sum.golang.org go tool goose -dir migrations postgres "$${DATABASE_URL}" down

migrate-status:
	set -a; . ./.env; set +a; GOSUMDB=sum.golang.org go tool goose -dir migrations postgres "$${DATABASE_URL}" status

run:
	set -a; . ./.env; set +a; GOSUMDB=sum.golang.org go run ./cmd/trip-service

build:
	GOSUMDB=sum.golang.org go build -o bin/trip-service ./cmd/trip-service

test:
	GOSUMDB=sum.golang.org go test -race ./...