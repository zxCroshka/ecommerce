.PHONY: lint lint-fix build test test-unit test-integration test-integration-product test-integration-order test-integration-notification test-race docker-up docker-down e2e-smoke migrate-create-userservice migrate-create-productservice migrate-create-orderservice migrate-create-notificationservice migrate-up-userservice migrate-down-userservice migrate-up-productservice migrate-down-productservice migrate-up-orderservice migrate-down-orderservice migrate-up-notificationservice migrate-down-notificationservice generate-proto generate-proto-userservice generate-proto-productservice generate-proto-cartservice generate-proto-orderservice generate-proto-notificationservice

build:
	go build ./...

lint:
	golangci-lint run ./... --build-tags=integration

lint-fix:
	golangci-lint run --fix ./...

migrate-create-userservice:
	migrate create -dir services/user-service/migrations/ -ext sql -seq init

migrate-create-productservice:
	migrate create -dir services/product-service/migrations/ -ext sql -seq init

migrate-create-orderservice:
	migrate create -dir services/order-service/migrations/ -ext sql -seq init

migrate-create-notificationservice:
	migrate create -dir services/notification-service/migrations/ -ext sql -seq init

migrate-up-userservice:
	migrate -path ./services/user-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=userservice_schema_migrations" up

migrate-down-userservice:
	migrate -path ./services/user-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=userservice_schema_migrations" down

migrate-up-productservice:
	migrate -path ./services/product-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=productservice_schema_migrations" up

migrate-down-productservice:
	migrate -path ./services/product-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=productservice_schema_migrations" down

migrate-up-orderservice:
	migrate -path ./services/order-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=orderservice_schema_migrations" up

migrate-down-orderservice:
	migrate -path ./services/order-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=orderservice_schema_migrations" down

migrate-up-notificationservice:
	migrate -path ./services/notification-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=notificationservice_schema_migrations" up

migrate-down-notificationservice:
	migrate -path ./services/notification-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=notificationservice_schema_migrations" down

test:
	go test ./...

test-unit:
	go test -short ./...

test-integration:
	go test -tags=integration -count=1 ./services/user-service/internal/repository/... ./services/product-service/internal/repository/postgres ./services/order-service/internal/repository ./services/notification-service/internal/repository

test-integration-product:
	go test -tags=integration -count=1 -v ./services/product-service/internal/repository/postgres

test-integration-order:
	go test -tags=integration -count=1 -v ./services/order-service/internal/repository

test-integration-notification:
	go test -tags=integration -count=1 -v ./services/notification-service/internal/repository

test-race:
	go test -race ./services/notification-service/... ./services/order-service/internal/service ./services/cart-service/internal/repository ./services/product-service/internal/repository/redis ./shared/outbox

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

e2e-smoke:
	./tests/e2e/smoke.sh

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

generate-proto-userservice:
	protoc -I ./shared/userservice/proto \
		./shared/userservice/proto/userservice.proto \
		--go_out=./shared/userservice/gen/go \
		--go_opt=paths=source_relative \
		--go-grpc_out=./shared/userservice/gen/go \
		--go-grpc_opt=paths=source_relative

generate-proto-productservice:
	protoc -I ./shared/productservice/proto \
		./shared/productservice/proto/productservice.proto \
		--go_out=./shared/productservice/gen/go \
		--go_opt=paths=source_relative \
		--go-grpc_out=./shared/productservice/gen/go \
		--go-grpc_opt=paths=source_relative



generate-proto-cartservice:
	protoc -I ./shared/cartservice/proto \
		./shared/cartservice/proto/cartservice.proto \
		--go_out=./shared/cartservice/gen/go \
		--go_opt=paths=source_relative \
		--go-grpc_out=./shared/cartservice/gen/go \
		--go-grpc_opt=paths=source_relative

generate-proto-orderservice:
	protoc -I ./shared/orderservice/proto \
		./shared/orderservice/proto/orderservice.proto \
		--go_out=./shared/orderservice/gen/go \
		--go_opt=paths=source_relative \
		--go-grpc_out=./shared/orderservice/gen/go \
		--go-grpc_opt=paths=source_relative

generate-proto-notificationservice:
	protoc -I ./shared/notificationservice/proto \
		./shared/notificationservice/proto/notificationservice.proto \
		--go_out=./shared/notificationservice/gen/go \
		--go_opt=paths=source_relative \
		--go-grpc_out=./shared/notificationservice/gen/go \
		--go-grpc_opt=paths=source_relative

generate-proto: generate-proto-userservice generate-proto-productservice generate-proto-cartservice generate-proto-orderservice generate-proto-notificationservice
