.PHONY: lint lint-fix migrate-create-userservice migrate-create-productservice test test-unit test-integration test-integration-product test-coverage build run lint-errcheck migrate-up test-grpc test-http generate-proto-userservice generate-proto-productservice generate-proto-cartservice run-productservice run-userservice build-productservice build-userservice migrate-up-userservice migrate-down-userservice migrate-up-productservice migrate-down-productservice test-postgres run-only

lint:
	golangci-lint run ./... --build-tags=integration

lint-fix:
	golangci-lint run --fix ./...

migrate-create-userservice:
	migrate create -dir services/user-service/migrations/ -ext sql -seq init

migrate-create-productservice:
	migrate create -dir services/product-service/migrations/ -ext sql -seq init

migrate-up-userservice:
	migrate -path ./services/user-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable" up

migrate-down-userservice:
	migrate -path ./services/user-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable" down

migrate-up-productservice:
	migrate -path ./services/product-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable" up

migrate-down-productservice:
	migrate -path ./services/product-service/migrations -database "postgres://postgres:postgres@localhost:5432/ecommerce?sslmode=disable" down

test:
	go test ./... -v

test-unit:
	go test ./internal/repository/users/... -v
	go test ./internal/repository/... -v -short

test-integration:
	go test ./internal/repository/... -v -tags=integration

test-integration-product:
	go test -tags=integration -count=1 -v ./services/product-service/internal/repository/postgres

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

test-postgres:
	docker-compose up -d postgres
	sleep 5
	go test ./internal/repository/... -v
	docker-compose down

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



build-userservice:
	cd services/user-service && go build -o bin/user-service cmd/main.go

build-productservice:
	cd services/product-service && go build -o bin/product-service cmd/main.go

run-userservice: build-userservice
	cd services/user-service && ./bin/user-service -config=./config/config.yaml

run-productservice: build-productservice
	cd services/product-service && ./bin/product-service -config=./config/config.yaml

run-only:
	./services/user-service/bin/user-service -config=./services/user-service/config/config.yaml

lint-errcheck:
	golangci-lint run --disable errcheck ./...

test-grpc:
	go test ./services/user-service/internal/grpc/... -v

test-http:
	go test ./services/user-service/internal/handlers/... -v
