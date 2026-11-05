.PHONY: generate migrate migrate-down migrate-status run build test test-integration lint

generate:
	GOSUMDB=sum.golang.org go tool oapi-codegen -generate types,chi-server -package api -include-operation-ids createTrip,getTrip,finishTrip,health,ready -o api/api.gen.go contracts/openapi/trip-service.openapi.yaml
	GOSUMDB=sum.golang.org go generate ./internal/usecase/... ./internal/http/trip/... ./internal/transport/http ./internal/infra/telemetry

migrate:
	set -a; . ./.env; set +a; GOSUMDB=sum.golang.org go tool goose -dir migrations postgres "$${DATABASE_URL}" up

migrate-down:
	set -a; . ./.env; set +a; GOSUMDB=sum.golang.org go tool goose -dir migrations postgres "$${DATABASE_URL}" down

migrate-status:
	set -a; . ./.env; set +a; GOSUMDB=sum.golang.org go tool goose -dir migrations postgres "$${DATABASE_URL}" status

run:
	set -a; . ./.env.example; . ./.env; set +a; GOSUMDB=sum.golang.org go run ./cmd/trip-service

build:
	GOSUMDB=sum.golang.org go build -o bin/trip-service ./cmd/trip-service

test:
	GOTOOLCHAIN=go1.26.0+auto GOSUMDB=sum.golang.org go test -race -count=2 -coverpkg=./internal/domain/...,./internal/usecase/... -coverprofile=coverage.out ./...
	GOTOOLCHAIN=go1.26.0+auto GOSUMDB=sum.golang.org go tool cover -func=coverage.out

test-integration:
	set -a; . ./.env; set +a; GOSUMDB=sum.golang.org go test -race -count=2 -tags=integration ./internal/repository/...

lint:
	GOSUMDB=sum.golang.org ./bin/golangci-lint run ./...