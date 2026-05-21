.PHONY: lint lint-fix migrate-create test test-unit test-integration test-coverage build run lint-errcheck migrate-up test-grpc test-http

lint:
	golangci-lint run ./... --build-tags=integration

lint-fix:
	golangci-lint run --fix ./...

migrate-create:
	 migrate create -dir services/user-service/migrations/ -ext sql -seq init
test:
	go test ./... -v

test-unit:
	go test ./internal/repository/users/... -v
	go test ./internal/repository/... -v -short

test-integration:
	go test ./internal/repository/... -v -tags=integration

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

test-postgres:
	docker-compose up -d postgres
	sleep 5
	go test ./internal/repository/... -v
	docker-compose down

generate-proto:
	protoc -I ./shared/userservice/proto \
		./shared/userservice/proto/userservice.proto \
		--go_out=./shared/userservice/gen/go \
		--go_opt=paths=source_relative \
		--go-grpc_out=./shared/userservice/gen/go \
		--go-grpc_opt=paths=source_relative



# Сборка бинарника user-service
build:
	cd services/user-service && go build -o bin/user-service cmd/main.go

# Запуск собранного бинарника
run: build
	cd services/user-service && ./bin/user-service -config=config/local.yml

# Или если хочешь запускать без пересборки каждый раз
run-only:
	./services/user-service/bin/user-service -config=./services/user-service/config/local.yml
lint-errcheck:
	golangci-lint run --disable errcheck ./...

migrate-up:
	migrate -path ./services/user-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable" up

make migrate-down:
	migrate -path ./services/user-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable" down
test-grpc:
	go test ./services/user-service/internal/grpc/... -v

test-http:
	go test ./services/user-service/internal/handlers/... -v
