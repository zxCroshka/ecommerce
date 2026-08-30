# Ecommerce Service

## Описание

Ecommerce Service — учебно-практический backend на Go из четырёх сервисов. Внешний HTTP API предоставляет API Gateway, а внутренние синхронные вызовы выполняются по gRPC. Реализованы аутентификация и профиль пользователя, каталог товаров, административные операции, корзина и внутреннее резервирование остатков. Полный сценарий оформления заказа пока отсутствует.

## Архитектура

```text
Client
  │ HTTP/JSON :8085
  ▼
API Gateway (Gin)
  ├── gRPC ──► User Service :9090
  │             ├── PostgreSQL: userservice.users
  │             ├── Redis DB 0: refresh tokens, blacklist
  │             └── Kafka producer: user.registered
  ├── gRPC ──► Product Service :9091
  │             ├── PostgreSQL: products, stock_reservations
  │             ├── Redis DB 1: кэш каталога
  │             ├── Kafka producer: product.updated
  │             └── gRPC ──► User Service: проверка admin
  └── gRPC ──► Cart Service :9093
                ├── Redis DB 2: корзины
                └── gRPC ──► Product Service: проверка товара

Kafka topics ──► consumers в репозитории отсутствуют
```

Gateway преобразует REST/JSON-запросы в gRPC-вызовы. PostgreSQL хранит пользователей, товары и резервации; Redis — auth state, кэш и корзины. Kafka сейчас используется только для публикации событий.

## Сервисы

### API Gateway

Точка входа в `services/gateway`: REST API на Gin, валидация DTO, извлечение Bearer token, проверка ролей, вызовы трёх gRPC-сервисов и преобразование gRPC errors в единый HTTP/JSON-формат. Middleware добавляют request ID, logging, recovery, authentication и role authorization.

### User Service

Сервис в `services/user-service` хранит пользователей через `pgxpool`, хеширует пароли с bcrypt, выпускает JWT access/refresh tokens, хранит refresh token IDs и access blacklist в Redis и публикует `user.registered`. Защищённые gRPC methods используют unary auth interceptor.

### Product Service

Сервис в `services/product-service` реализует каталог, admin mutations, Redis cache-aside и PostgreSQL-резервации остатков. Admin проверяется через User Service; внутренние `ReserveStock`/`ReleaseStock` защищены service token. События части изменений публикуются в `product.updated`, но готового Order Service, вызывающего stock methods, нет.

### Cart Service

Сервис в `services/cart-service` хранит корзины в Redis hashes с TTL, проверяет товар через Product Service и атомарно меняет количество Lua-скриптами. Внутренний gRPC checkout возвращает и очищает корзину, но не подключён к HTTP API. Входящий Cart gRPC API пока доверяет `user_id` из запроса и не имеет auth interceptor.

## Реализованная функциональность

- регистрация, login, logout и изменение профиля;
- bcrypt password hashing;
- JWT access/refresh tokens, JTI, TTL и атомарная refresh rotation;
- Redis blacklist текущего access token при logout;
- публичное чтение, фильтрация, сортировка и offset pagination товаров;
- admin create, partial update и soft delete товаров;
- Redis cache-aside для каталога;
- чтение и изменение корзины с атомарными Lua operations;
- транзакционные идемпотентные `ReserveStock` и `ReleaseStock`;
- gRPC-контракты User, Product и Cart;
- Kafka producers для `user.registered` и `product.updated`;
- unit и integration tests.

Частично реализованы Kafka (нет consumers/outbox/DLQ), checkout (нет REST route и Order flow) и graceful shutdown (User и Product явно не закрывают PostgreSQL/Redis clients).

## Технологии

| Технология | Использование |
| --- | --- |
| Go 1.25 | Сервисы и общие контракты |
| Gin | HTTP API Gateway; pprof User Service |
| PostgreSQL 16 | Пользователи, товары, stock reservations |
| pgx / pgxpool | SQL, connection pools, transactions |
| Redis 7 | Auth state, кэш каталога, корзины |
| Lua | Атомарная refresh rotation и операции корзины |
| gRPC / protobuf | Синхронные межсервисные вызовы |
| Kafka | Публикация событий User и Product Service |
| JWT / bcrypt | Аутентификация и хранение паролей |
| Docker Compose | Инфраструктура, миграции и запуск |
| slog | Structured logging |
| testify / miniredis / testcontainers-go | Тесты |

## Структура проекта

```text
.
├── services/
│   ├── gateway/{cmd,config,internal,tests/load}/
│   ├── user-service/{cmd,config,internal,migrations}/
│   ├── product-service/{cmd,config,internal,migrations}/
│   └── cart-service/{cmd,config,internal}/
├── shared/
│   ├── userservice/{proto,gen/go}/
│   ├── productservice/{proto,gen/go}/
│   └── cartservice/{proto,gen/go}/
├── docs/
├── docker-compose.yml
├── Makefile
├── go.mod
└── go.sum
```

`cmd` содержит точки входа, `internal` — transport, service и repository layers, `shared/*/proto` — контракты, `shared/*/gen/go` — generated code, `migrations` — SQL-миграции, `tests/load` — сценарии `wrk`.

## HTTP API

| Метод | Endpoint | Назначение | Auth |
| --- | --- | --- | --- |
| GET | `/healthz` | Liveness Gateway | Нет |
| POST | `/api/v1/auth/register` | Регистрация | Нет |
| POST | `/api/v1/auth/login` | Получение токенов | Нет |
| POST | `/api/v1/auth/refresh` | Rotation токенов | Refresh token в body |
| POST | `/api/v1/auth/logout` | Logout сессии | Bearer |
| GET | `/api/v1/users/me` | Текущий профиль | Bearer |
| PATCH | `/api/v1/users/me/email` | Изменить email | Bearer |
| PATCH | `/api/v1/users/me/name` | Изменить имя | Bearer |
| PATCH | `/api/v1/users/me/password` | Изменить пароль | Bearer |
| GET | `/api/v1/products` | Список товаров | Нет |
| GET | `/api/v1/products/:id` | Получить товар | Нет |
| POST | `/api/v1/products` | Создать товар | Admin |
| PATCH | `/api/v1/products/:id` | Изменить товар | Admin |
| DELETE | `/api/v1/products/:id` | Soft delete | Admin |
| GET | `/api/v1/cart` | Получить корзину | Bearer |
| POST | `/api/v1/cart/items` | Добавить позицию | Bearer |
| PATCH | `/api/v1/cart/items/:product_id` | Установить количество | Bearer |
| DELETE | `/api/v1/cart/items/:product_id` | Удалить позицию | Bearer |

Ошибки возвращаются в едином формате: `{"success":false,"error":{"code":"...","message":"..."}}`.

## Аутентификация

После login клиент получает access и refresh token. Access token передаётся как `Authorization: Bearer <token>`. JWT подписываются HS256 и содержат `user_id`, `email`, `role`, `token_type`, `jti`, `iat`, `exp`; TTL по умолчанию — 15 минут и 168 часов.

Refresh token ID хранится в Redis. Lua script атомарно заменяет его при refresh. Logout удаляет одну refresh-сессию и добавляет текущий access token в blacklist до истечения TTL; другие сессии пользователя не отзываются.

## PostgreSQL

User и Product Service используют разные schemas одной базы:

- `userservice.users` — пользователь, unique email, password hash, role;
- `productservice.products` — товар, цена, остаток, категория, active status;
- `productservice.stock_reservations` — reservation ID, товар, количество, status.

Подключения создаются через `pgxpool`, при старте выполняется `Ping`. Регистрация и stock operations используют transactions. `ReleaseStock` блокирует reservation через `SELECT ... FOR UPDATE`, предотвращая повторный возврат остатка при конкурентных запросах.

## Redis и Lua

- DB 0 — refresh token IDs и access blacklist;
- DB 1 — Product cache-aside с TTL 5 минут;
- DB 2 — `cart:{user_id}` hashes с TTL 168 часов.

PostgreSQL остаётся source of truth для каталога, Redis — единственное runtime-хранилище корзин. Lua объединяет несколько Redis-команд в атомарные refresh/cart operations и исключает обычный read-modify-write race.

## gRPC

Контракты в `shared/*/proto` описывают authentication/profile, catalog CRUD и stock reservations, а также cart operations и checkout. Gateway вызывает все три сервиса; Cart обращается к Product, Product — к User для проверки admin. Gateway переводит gRPC status codes в HTTP statuses.

## Kafka

User Service публикует `user.registered`, Product Service — `product.updated` при части изменений товара и stock operations. Kafka key содержит user/product ID. Названия topics в producer code фиксированы. Consumers, offset handling, application retries, outbox и DLQ отсутствуют, поэтому интеграция producer-only и частичная.

## Конфигурация

YAML-файлы находятся в `services/*/config`; значения переопределяются переменными с префиксом `APP_`.

| Переменная | Назначение |
| --- | --- |
| `APP_POSTGRES_HOST`, `APP_POSTGRES_PASSWORD` | PostgreSQL User/Product |
| `APP_REDIS_HOST`, `APP_REDIS_PASSWORD` | Redis |
| `APP_KAFKA_BROKERS` | Kafka brokers User/Product |
| `APP_JWT_SECRET` | JWT signing secret User Service |
| `APP_USER_SERVICE_ADDRESS` | User gRPC для Gateway/Product |
| `APP_PRODUCT_SERVICE_ADDRESS` | Product gRPC для Gateway/Cart |
| `APP_CART_SERVICE_ADDRESS` | Cart gRPC для Gateway |
| `APP_GRPC_INTERNAL_TOKEN` | Проверка внутренних Product methods |
| `APP_PRODUCT_SERVICE_INTERNAL_TOKEN` | Такой же token в Gateway client |

Реальные secrets нельзя хранить в Git; `.env.example` пока отсутствует.

## Запуск через Docker Compose

Требуется Docker с Compose plugin.

```bash
docker compose up -d --build
docker compose ps
curl -fsS http://localhost:8085/healthz
```

Compose запускает PostgreSQL, Redis, ZooKeeper, Kafka, Kafka UI, migrations и четыре приложения. User/Product migrations выполняются до старта сервисов. Gateway доступен на `localhost:8085`, Kafka UI — на `localhost:8088`, gRPC — на портах `9090`, `9091`, `9093`.

```bash
docker compose logs -f api-gateway user-service product-service cart-service
docker compose down
```

## Запуск без Docker

Нужны Go 1.25, PostgreSQL, Redis, Kafka и CLI `migrate`. Создайте БД `ecommerce`, примените обе миграции командами из следующего раздела, затем запустите процессы в разных терминалах:

```bash
(cd services/user-service && APP_POSTGRES_PASSWORD='YOUR_PASSWORD' APP_JWT_SECRET='LOCAL_SECRET' go run ./cmd)
(cd services/product-service && APP_POSTGRES_PASSWORD='YOUR_PASSWORD' APP_GRPC_INTERNAL_TOKEN='LOCAL_SERVICE_TOKEN' go run ./cmd)
(cd services/cart-service && go run ./cmd)
(cd services/gateway && APP_USER_SERVICE_ADDRESS='127.0.0.1:9090' APP_PRODUCT_SERVICE_ADDRESS='127.0.0.1:9091' APP_CART_SERVICE_ADDRESS='127.0.0.1:9093' APP_PRODUCT_SERVICE_INTERNAL_TOKEN='LOCAL_SERVICE_TOKEN' go run ./cmd)
```

Локальные YAML-конфиги ожидают PostgreSQL, Redis и Kafka на localhost; значения двух internal token должны совпадать.

## Миграции

SQL находится в `services/user-service/migrations` и `services/product-service/migrations`. Compose выполняет его containers `migrate-user` и `migrate-product`. Вручную:

```bash
migrate -path ./services/user-service/migrations -database 'postgres://postgres:YOUR_PASSWORD@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=userservice_schema_migrations' up
migrate -path ./services/product-service/migrations -database 'postgres://postgres:YOUR_PASSWORD@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=productservice_schema_migrations' up
```

После изменения proto: `make generate-proto-userservice`, `make generate-proto-productservice` или `make generate-proto-cartservice`.

## Тесты

Есть unit tests service/gRPC/Gateway handler/middleware/JWT-кода, Redis repository tests на `miniredis`, PostgreSQL integration tests через `testcontainers-go`, Kafka integration tests с tag `integration` и `wrk` load scripts.

```bash
# Для полного прогона нужен работающий Docker daemon: User repository tests запускают testcontainers.
go test -count=1 ./...

# Product PostgreSQL integration tests.
go test -tags=integration -count=1 ./services/product-service/internal/repository/postgres

# User Kafka integration tests; Kafka должна быть доступна на localhost:9092.
go test -tags=integration -count=1 ./services/user-service/internal/kafka
```

## Примеры запросов

```bash
# Регистрация
curl -i -X POST http://localhost:8085/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"password123","name":"Test User"}'

# Login и access token
LOGIN_RESPONSE=$(curl -sS -X POST http://localhost:8085/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"password123"}')
ACCESS_TOKEN=$(printf '%s' "$LOGIN_RESPONSE" | jq -r '.data.access_token')

# Защищённый профиль
curl -i http://localhost:8085/api/v1/users/me \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

## Статус проекта

Проект находится в разработке. Рабочий контур включает Gateway, User, Product и Cart Service: authentication, профиль, каталог, admin mutations, корзину и внутренние stock reservations. Order Service и REST checkout отсутствуют; Notification и Payment Service также не реализованы.

## Ограничения текущей реализации

- Cart gRPC доверяет переданному `user_id`;
- Kafka работает без consumers и transactional outbox;
- асинхронный Product cache допускает временно устаревшие данные;
- User и Product явно не закрывают PostgreSQL/Redis clients при shutdown;
- у части mutation gRPC-вызовов нет отдельного application deadline;
- нет rate limiting, CORS, metrics и distributed tracing;
- только Gateway имеет простой `/healthz`, readiness внутренних сервисов нет;
- автоматический end-to-end тест не добавлен в репозиторий.
