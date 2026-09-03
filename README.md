# Ecommerce Service

## Описание

Ecommerce Service — учебно-практический backend на Go из шести приложений. Внешний HTTP API предоставляет API Gateway, внутренние синхронные вызовы выполняются по gRPC, а доменные события доставляются через Kafka. Реализованы аутентификация и профиль пользователя, каталог, корзина, идемпотентное оформление заказа без оплаты и пользовательские уведомления.

## Архитектура

```text
Client
  │ HTTP/JSON :8085
  ▼
API Gateway (Gin)
  ├── gRPC ──► User Service :9090
  │             ├── PostgreSQL: userservice.users
  │             ├── Redis DB 0: refresh tokens, blacklist
  │             └── PostgreSQL outbox relay → Kafka: user.registered
  ├── gRPC ──► Product Service :9091
  │             ├── PostgreSQL: products, stock_reservations
  │             ├── Redis DB 1: кэш каталога
  │             ├── PostgreSQL outbox relay → Kafka: product.updated
  │             └── gRPC ──► User Service: проверка admin
  ├── gRPC ──► Cart Service :9093
  │             ├── Redis DB 2: корзины и revision
  │             └── gRPC ──► Product Service: проверка товара
  ├── gRPC ──► Order Service :9094
  │             ├── PostgreSQL: orders, order_items, outbox
  │             ├── gRPC ──► Cart/Product: snapshot, reserve, conditional clear
  │             └── PostgreSQL outbox relay → Kafka: order.created
  └── gRPC ──► Notification Service :9095
                ├── PostgreSQL: notifications (UNIQUE event_id)
                ├── gRPC ──► User Service: проверка bearer
                └── Kafka consumer group ◄── user.registered, order.created
```

Gateway преобразует REST/JSON-запросы в gRPC-вызовы. PostgreSQL хранит пользователей, товары, резервации, заказы, outbox и notifications; Redis — auth state, кэш и корзины. Kafka связывает transactional outbox producer'ы с Notification consumer.

## Сервисы

### API Gateway

Точка входа в `services/gateway`: REST API на Gin, валидация DTO, извлечение Bearer token, проверка ролей, вызовы внутренних gRPC-сервисов и преобразование gRPC errors в единый HTTP/JSON-формат. Middleware добавляют единый request deadline, request ID, logging, recovery, authentication и role authorization.

### User Service

Сервис в `services/user-service` хранит пользователей через `pgxpool`, хеширует пароли с bcrypt, выпускает JWT access/refresh tokens, хранит refresh token IDs и access blacklist в Redis. Регистрация атомарно сохраняет пользователя и `user.registered` в transactional outbox. Защищённые gRPC methods используют unary auth interceptor.

### Product Service

Сервис в `services/product-service` реализует каталог, admin mutations, Redis cache-aside с generation fence и PostgreSQL-резервации остатков. Admin проверяется через User Service; внутренние идемпотентные `ReserveStock`/`ReleaseStock` защищены service token. Изменения, для которых существует `product.updated`, сохраняют event в outbox в той же транзакции.

### Cart Service

Сервис в `services/cart-service` хранит корзины в Redis hashes с TTL, проверяет товар через Product Service и атомарно меняет количество и revision Lua-скриптами. User-facing gRPC methods валидируют Bearer token на границе сервиса и берут identity из context. Отдельно защищённые internal methods возвращают non-destructive snapshot с revision и условно очищают только неизменившуюся корзину.

### Order Service

Сервис в `services/order-service` сохраняет price snapshot заказа в PostgreSQL, оркестрирует детерминированные Product reservations и compensation, восстанавливает старые `pending` orders и атомарно фиксирует `confirmed` вместе с outbox event `order.created`. Create/Get/List берут user identity только из проверенного bearer context.

### Notification Service

Сервис в `services/notification-service` состоит из managed Kafka consumer и защищённого gRPC API. Consumer group читает `user.registered` и `order.created`, проверяет event type/version и сохраняет notification в собственной PostgreSQL schema. `UNIQUE(event_id)` делает повторную доставку безопасной; offsets commit'ятся только после durable insert или подтверждённого duplicate.

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
- идемпотентное оформление заказа с price snapshot, compensation и conditional cart clear;
- gRPC-контракты User, Product, Cart и Order;
- transactional outbox и managed Kafka relay для `user.registered`, `product.updated` и `order.created`;
- Kafka consumer с manual offset commit, bounded retry/backoff и идемпотентной записью notifications;
- получение своих уведомлений и идемпотентная отметка `read_at`;
- unit и integration tests.

Payment Service не реализован. DLQ для permanently invalid Kafka events не реализована: такие события явно логируются и пропускаются, а временные ошибки БД не commit'ятся. Сервисы владеют своими worker'ами, producer/consumer, PostgreSQL/Redis и gRPC connections и закрывают их при shutdown.

## Технологии

| Технология | Использование |
| --- | --- |
| Go 1.25 | Сервисы и общие контракты |
| Gin | HTTP API Gateway; pprof User Service |
| PostgreSQL 16 | Пользователи, товары, stock reservations, orders, service-owned outbox tables |
| pgx / pgxpool | SQL, connection pools, transactions |
| Redis 7 | Auth state, кэш каталога, корзины |
| Lua | Атомарная refresh rotation и операции корзины |
| gRPC / protobuf | Синхронные межсервисные вызовы |
| Kafka | Outbox delivery и Notification consumer group |
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
│   ├── cart-service/{cmd,config,internal}/
│   ├── order-service/{cmd,config,internal,migrations}/
│   └── notification-service/{cmd,config,internal,migrations}/
├── shared/
│   ├── userservice/{proto,gen/go}/
│   ├── productservice/{proto,gen/go}/
│   ├── cartservice/{proto,gen/go}/
│   ├── orderservice/{proto,gen/go}/
│   ├── notificationservice/{proto,gen/go}/
│   └── outbox/
├── docs/INTERVIEW_PROJECT_SUMMARY_RU.md
├── tests/e2e/smoke.sh
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
| POST | `/api/v1/orders` | Создать/продолжить заказ (`Idempotency-Key`) | Bearer |
| GET | `/api/v1/orders` | Список своих заказов | Bearer |
| GET | `/api/v1/orders/:id` | Получить свой заказ | Bearer |
| GET | `/api/v1/notifications` | Список своих уведомлений | Bearer |
| PATCH | `/api/v1/notifications/:id/read` | Отметить своё уведомление прочитанным | Bearer |

Ошибки возвращаются в едином формате: `{"success":false,"error":{"code":"...","message":"..."}}`.

## Аутентификация

После login клиент получает access и refresh token. Access token передаётся как `Authorization: Bearer <token>`. JWT подписываются HS256 и содержат `user_id`, `email`, `role`, `token_type`, `jti`, `iat`, `exp`; TTL по умолчанию — 15 минут и 168 часов.

Refresh token ID хранится в Redis. Lua script атомарно заменяет его при refresh. Logout удаляет одну refresh-сессию и добавляет текущий access token в blacklist до истечения TTL; другие сессии пользователя не отзываются.

## PostgreSQL

User, Product, Order и Notification Service используют разные schemas одной базы:

- `userservice.users` — пользователь, unique email, password hash, role;
- `productservice.products` — товар, цена, остаток, категория, active status;
- `productservice.stock_reservations` — reservation ID, товар, количество, status.
- `orderservice.orders`, `orderservice.order_items` — заказ и неизменяемые snapshots его позиций;
- `notificationservice.notifications` — уведомления с уникальным source `event_id` и nullable `read_at`.

Подключения создаются через `pgxpool`, при старте выполняется `Ping`. Регистрация и stock operations используют transactions. `ReleaseStock` блокирует reservation через `SELECT ... FOR UPDATE`, предотвращая повторный возврат остатка при конкурентных запросах.

## Redis и Lua

- DB 0 — refresh token IDs и access blacklist;
- DB 1 — Product cache-aside с TTL 5 минут;
- DB 2 — `cart:{user_id}` hashes с TTL 168 часов.

PostgreSQL остаётся source of truth для каталога, Redis — единственное runtime-хранилище корзин. Lua объединяет несколько Redis-команд в атомарные refresh/cart operations и исключает обычный read-modify-write race.

## gRPC

Контракты в `shared/*/proto` описывают authentication/profile, catalog CRUD, stock reservations, cart snapshot, orders и notifications. Cart обращается к Product, Order — к Cart/Product, а защищённые сервисы — к User для проверки bearer token. Gateway переводит gRPC status codes в HTTP statuses.

## Kafka

User, Product и Order Service публикуют `user.registered`, `product.updated` и `order.created` через transactional outbox. Изменение business state и outbox row commit'ятся вместе; managed relay публикует versioned envelope с `event_id`, ждёт delivery report и помечает row published.

Notification consumer group читает `user.registered` и `order.created` без auto-commit. После durable `INSERT ... ON CONFLICT(event_id) DO NOTHING` offset commit'ится вручную. Поэтому транспорт остаётся at-least-once, а эффект создания notification — effectively-once. Неизвестная версия или malformed event явно логируется и commit'ится, чтобы не блокировать partition; DLQ остаётся дальнейшим улучшением. Временная ошибка PostgreSQL получает bounded retry, затем процесс завершается без commit, и сообщение повторно доставляется после restart.

## Конфигурация

YAML-файлы находятся в `services/*/config`; значения переопределяются переменными с префиксом `APP_`.

| Переменная | Назначение |
| --- | --- |
| `APP_POSTGRES_HOST`, `APP_POSTGRES_PASSWORD` | PostgreSQL User/Product/Order/Notification |
| `APP_REDIS_HOST`, `APP_REDIS_PASSWORD` | Redis |
| `APP_KAFKA_BROKERS` | Kafka brokers producer'ов и Notification consumer |
| `APP_JWT_SECRET` | JWT signing secret User Service |
| `APP_USER_SERVICE_ADDRESS` | User gRPC для Gateway и service auth boundaries |
| `APP_PRODUCT_SERVICE_ADDRESS` | Product gRPC для Gateway/Cart/Order |
| `APP_CART_SERVICE_ADDRESS` | Cart gRPC для Gateway/Order |
| `APP_ORDER_SERVICE_ADDRESS` | Order gRPC для Gateway |
| `APP_NOTIFICATION_SERVICE_ADDRESS` | Notification gRPC для Gateway |
| `APP_CART_SERVICE_INTERNAL_TOKEN` | Order → Cart internal credential |
| `APP_GRPC_INTERNAL_TOKEN` | Проверка внутренних Product/Cart methods в соответствующем сервисе |
| `APP_PRODUCT_SERVICE_INTERNAL_TOKEN` | Product credential для Gateway/Order client |

Cart user-facing methods валидируют bearer token на собственной gRPC-границе и используют identity из context. Внутренние snapshot/conditional-clear методы защищены отдельным `x-service-token`; для production остаётся заменить application-level shared secrets и plaintext transport на mTLS/workload identity.

Реальные secrets нельзя хранить в Git; `.env.example` пока отсутствует.

## Запуск через Docker Compose

Требуется Docker с Compose plugin.

```bash
docker compose up -d --build
docker compose ps
curl -fsS http://localhost:8085/healthz
```

Compose запускает PostgreSQL, Redis, ZooKeeper, Kafka, Kafka UI, migrations и шесть приложений. User/Product/Order/Notification migrations выполняются до старта сервисов, а healthchecks не позволяют downstream стартовать лишь по факту создания контейнера. Gateway доступен на `localhost:8085`, Kafka UI — на `localhost:8088`, gRPC — на портах `9090`, `9091`, `9093`, `9094`, `9095`.

```bash
docker compose logs -f api-gateway user-service product-service cart-service order-service notification-service
docker compose down
```

## Запуск без Docker

Нужны Go 1.25, PostgreSQL, Redis, Kafka и CLI `migrate`. Создайте БД `ecommerce`, примените миграции всех PostgreSQL-сервисов командами из следующего раздела, затем запустите процессы в разных терминалах:

```bash
(cd services/user-service && APP_POSTGRES_PASSWORD='YOUR_PASSWORD' APP_JWT_SECRET='LOCAL_SECRET' go run ./cmd)
(cd services/product-service && APP_POSTGRES_PASSWORD='YOUR_PASSWORD' APP_GRPC_INTERNAL_TOKEN='LOCAL_SERVICE_TOKEN' go run ./cmd)
(cd services/cart-service && go run ./cmd)
(cd services/order-service && APP_POSTGRES_PASSWORD='YOUR_PASSWORD' APP_CART_SERVICE_INTERNAL_TOKEN='LOCAL_CART_TOKEN' APP_PRODUCT_SERVICE_INTERNAL_TOKEN='LOCAL_PRODUCT_TOKEN' go run ./cmd)
(cd services/notification-service && APP_POSTGRES_PASSWORD='YOUR_PASSWORD' go run ./cmd)
(cd services/gateway && APP_USER_SERVICE_ADDRESS='127.0.0.1:9090' APP_PRODUCT_SERVICE_ADDRESS='127.0.0.1:9091' APP_CART_SERVICE_ADDRESS='127.0.0.1:9093' APP_ORDER_SERVICE_ADDRESS='127.0.0.1:9094' APP_NOTIFICATION_SERVICE_ADDRESS='127.0.0.1:9095' APP_PRODUCT_SERVICE_INTERNAL_TOKEN='LOCAL_PRODUCT_TOKEN' go run ./cmd)
```

Локальные YAML-конфиги ожидают PostgreSQL, Redis и Kafka на localhost; значения двух internal token должны совпадать.

## Миграции

SQL находится в `services/*/migrations`. Compose выполняет containers `migrate-user`, `migrate-product`, `migrate-order` и `migrate-notification`. Вручную:

```bash
migrate -path ./services/user-service/migrations -database 'postgres://postgres:YOUR_PASSWORD@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=userservice_schema_migrations' up
migrate -path ./services/product-service/migrations -database 'postgres://postgres:YOUR_PASSWORD@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=productservice_schema_migrations' up
migrate -path ./services/order-service/migrations -database 'postgres://postgres:YOUR_PASSWORD@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=orderservice_schema_migrations' up
migrate -path ./services/notification-service/migrations -database 'postgres://postgres:YOUR_PASSWORD@localhost:5432/ecommerce?sslmode=disable&x-migrations-table=notificationservice_schema_migrations' up
```

После изменения proto используйте `make generate-proto` либо service-specific target, включая `make generate-proto-notificationservice`.

## Тесты

Есть unit tests service/gRPC/Gateway handler/middleware/JWT/outbox/consumer-кода, Redis repository tests на `miniredis`, PostgreSQL integration tests через `testcontainers-go`, Kafka integration tests с tag `integration`, Compose E2E smoke и `wrk` load scripts.

```bash
# Unit и package tests без testcontainers.
make test

# PostgreSQL integration tests через testcontainers.
make test-integration

# Race subset и smoke после docker-up.
make test-race
make docker-up
make e2e-smoke
make docker-down
```

`tests/e2e/smoke.sh` требует `curl`, `jq` и Docker Compose. Он создаёт уникального пользователя, повышает его роль только в локальной test DB, создаёт товар/корзину/заказ, проверяет stock и повтор с тем же idempotency key, затем ждёт welcome/order notification и помечает её прочитанной.

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

# Создать заказ по текущему snapshot корзины
curl -i -X POST http://localhost:8085/api/v1/orders \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Idempotency-Key: checkout-001'

# Получить уведомления
curl -i http://localhost:8085/api/v1/notifications \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

## Статус проекта

Проект находится в разработке. Рабочий демонстрационный контур включает Gateway, User, Product, Cart, Order и Notification Service: authentication, профиль, каталог, корзину, stock reservations, REST checkout и Kafka-driven notifications. Payment Service не реализован.

## Ограничения текущей реализации

- DLQ и operator replay tooling для permanently invalid Kafka events отсутствуют;
- internal service authentication пока использует shared secrets и plaintext gRPC; для production нужны workload identity/mTLS;
- при недоступном Redis Product Service использует PostgreSQL как source of truth, но старые cache entries исчезают только по TTL;
- нет rate limiting, CORS, metrics и distributed tracing;
- readiness gRPC-сервисов в Compose проверяет открытый TCP port, а не глубокую доступность всех downstream;
- E2E smoke рассчитан на локальный Compose и использует прямой SQL fixture для выдачи admin-роли.
