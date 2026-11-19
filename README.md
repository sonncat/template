# Trip Service

Учебный сервис поездок на Go и PostgreSQL. Сервис создаёт, получает и завершает поездки, сохраняет координаты и возвращает маршрут. Если активная поездка долго не присылает координаты, сервис запрашивает их через Push Service.

## Запуск

Для запуска нужны Go, Docker и `tripgoctl`. Версия Go указана в `go.mod`. Для примеров ниже также нужны `curl` и Python 3.

Из корня проекта поднимите окружение третьей лабораторной работы:

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
```

Состав окружения указан в `environment.toml`: PostgreSQL, средства наблюдаемости и Push Service. `tripgoctl environment start` создаёт локальный `.env` с адресами компонентов, а `tripgoctl connect` показывает эти адреса.

Примените миграции и запустите сервис:

```bash
make migrate
make migrate-status
make run
```

`make run` сначала загружает `.env.example`, затем `.env`, поэтому локальные значения имеют приоритет. Проверьте, что `PUSH_HTTP_URL` в `.env` соответствует адресу Push HTTP из `tripgoctl connect`: адрес в `.env.example` приведён только как пример. Локальный `.env` не добавляется в Git.

Адрес сервиса задаётся `HTTP_ADDR`. В примерах используется `localhost:8080`:

```bash
curl -i http://localhost:8080/health
curl -i http://localhost:8080/ready
```

`/health` показывает, что процесс работает; `/ready` также проверяет доступность базы данных.

## HTTP API

Контракт находится в `contracts/openapi/trip-service.openapi.yaml`. Основные операции:

| Метод и путь | Действие |
|---|---|
| `POST /api/v1/trips` | Создать поездку |
| `GET /api/v1/trips/{tripId}` | Получить поездку |
| `POST /api/v1/trips/{tripId}/finish` | Завершить поездку |
| `POST /api/v1/trips/{tripId}/positions` | Сохранить координату |
| `GET /api/v1/trips/{tripId}/positions` | Получить маршрут |

Для проверки создайте поездку с новым идентификатором водителя:

```bash
DRIVER_ID=$(python3 -c 'import uuid; print(uuid.uuid4())')

RESPONSE=$(curl -fsS -X POST http://localhost:8080/api/v1/trips \
  -H 'Content-Type: application/json' \
  -d "{\"user_id\":\"5cb72c04-7650-45c9-a79b-bcdba0631e0c\",\"driver_id\":\"$DRIVER_ID\",\"start_point\":{\"latitude\":59.9398,\"longitude\":30.3146},\"end_point\":{\"latitude\":59.9290,\"longitude\":30.3626},\"price\":1450}")

printf '%s\n' "$RESPONSE"
TRIP_ID=$(printf '%s' "$RESPONSE" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
printf 'TRIP_ID=%s\n' "$TRIP_ID"
```

Сохраните координату и получите маршрут:

```bash
curl -i -X POST "http://localhost:8080/api/v1/trips/$TRIP_ID/positions" \
  -H 'Content-Type: application/json' \
  -d "{\"coordinates\":{\"latitude\":59.93,\"longitude\":30.33},\"recorded_at\":\"$(date -u +%FT%TZ)\"}"

curl -i "http://localhost:8080/api/v1/trips/$TRIP_ID/positions"
```

Маршрут возвращает все точки в порядке `recorded_at`, а при совпадении времени — по `id`. Для несуществующей поездки возвращается `404 trip_not_found`; сохранить координату завершённой поездки нельзя — возвращается `409 trip_completed`. Некорректные координаты, дата или дата более чем на минуту в будущем дают `400 invalid_request`.

Получить и завершить созданную поездку:

```bash
curl -i "http://localhost:8080/api/v1/trips/$TRIP_ID"
curl -i -X POST "http://localhost:8080/api/v1/trips/$TRIP_ID/finish"
```

Ошибки возвращаются в формате `application/problem+json` со стабильным полем `code`. Внутренние детали ошибок базы в ответ не попадают.

## Архитектура

Зависимости собираются в `internal/container`. HTTP-обработчики вызывают usecase, usecase работает с доменной моделью и интерфейсами репозитория и менеджера транзакций. Реализации репозитория и инфраструктуры работают с PostgreSQL.

- `internal/http/trip` и `internal/http/position` — HTTP-обработчики поездок и координат
- `internal/transport/http` — адаптер сгенерированного OpenAPI-интерфейса, health и ready
- `internal/usecase/trip` и `internal/usecase/position` — сценарии работы сервиса
- `internal/domain` — модель и доменные правила
- `internal/repository/trip` — запросы к данным поездок и координат
- `internal/infra/postgres` — подключение к базе, транзакции и инструментирование запросов
- `internal/worker/positions` — сканер устаревших поездок, очередь и воркеры
- `internal/gateway/push` — HTTP-клиент Push Service
- `internal/infra/telemetry` — провайдеры трасс и метрик

Интерфейсы зависимостей объявлены на стороне потребителя. OpenAPI ожидает один объект со всеми методами, поэтому адаптер в `internal/transport/http` передаёт вызовы отдельным обработчикам. После переноса директорий Go-пакеты PostgreSQL, транзакций и репозитория сохранили имена `db`, `txmanager` и `repository`.

## Транзакции и конкурентные запросы

Для транзакций явно выбран PostgreSQL `Read Committed`. Он позволяет работать с зафиксированными данными, а защиту конкретных операций обеспечивают ограничения и запросы базы:

- частичный уникальный индекс не допускает две активные поездки одного водителя
- условный `UPDATE` по активному статусу не позволяет повторно завершить поездку
- сохранение координаты обновляет строку активной поездки и блокирует её до конца транзакции, поэтому параллельное завершение не проходит между проверкой статуса и вставкой координаты

Создание поездки и записи её истории выполняются в одной транзакции. Сохранение координаты и обновление `last_position_at` тоже выполняются вместе через `TxManager`. Время сравнивается в SQL с помощью `GREATEST`: старая точка сохраняется в маршруте, но не уменьшает время последней координаты. Вложенный вызов менеджера переиспользует транзакцию из контекста.

## Опрос устаревших поездок

```text
сканер → ограниченная очередь → воркеры → Push Service
```

Один сканер по тикеру делает SQL-запрос к базе. Он выбирает активные поездки, у которых с последней координаты прошло больше `POSITION_STALE_AFTER`. Если координат ещё не было, время отсчитывается от `started_at`. Сначала идут поездки, которые молчат дольше. Для этого запроса добавлен частичный индекс по `COALESCE(last_position_at, started_at)` и `id`.

Задачи попадают в буферизованный канал, который разбирает фиксированное число воркеров. Горутина на каждую поездку не создаётся. Защищённая мьютексом структура не позволяет повторно поставить поездку, пока её задача ожидает или выполняется. Если координата пришла до начала обработки, воркер пропустит отменённую задачу. Отдельно учитывается координата, пришедшая во время текущей выборки, до постановки задачи в очередь.

Сканер не ждёт свободного места. При полной очереди задача отбрасывается, записывается предупреждение и увеличивается `tripgo.stale_checks` с результатом `dropped`. Поездка остаётся в базе и может снова попасть в выборку на следующем тике. Каждый вызов Push Service выполняется один раз с таймаутом; ретраев и rate limiter в этой работе нет.

Выборка намеренно идёт без `LIMIT`, а очередь работает по FIFO: первой из неё берут задачу, которую первой поставили. Поэтому большая выборка может не поместиться в очередь, а между тиками общий порядок по давности не гарантирован. Время последнего push отдельно не хранится: если ответа нет, поездка может снова попасть в выборку на следующем тике. Эти упрощения оставлены по условию базовой части работы.

## Конфигурация

Пример находится в `.env.example`. Адреса конкретного окружения берутся из `.env` после `tripgoctl connect`.

| Переменная | Значение по умолчанию | Назначение |
|---|---:|---|
| `POSITION_CHECK_INTERVAL` | `30s` | Период проверки устаревших поездок |
| `POSITION_STALE_AFTER` | `60s` | Время без координат, после которого поездка считается устаревшей |
| `POSITION_WORKERS` | `4` | Количество воркеров |
| `POSITION_QUEUE_SIZE` | `100` | Вместимость очереди |
| `PUSH_REQUEST_TIMEOUT` | `1s` | Таймаут вызова Push Service |
| `PUSH_HTTP_URL` | Адрес окружения в `.env` | HTTP-адрес Push Service |

Остальные параметры: `HTTP_ADDR` и `HTTP_*_TIMEOUT` задают HTTP-сервер, `DATABASE_*` — подключение и таймауты базы, `SHUTDOWN_TIMEOUT` — общий бюджет остановки, `LOG_LEVEL` — уровень логов, `OTEL_*` — параметры OpenTelemetry. `DATABASE_QUERY_TIMEOUT` также ограничивает запрос сканера к базе.

## Телеметрия

Сервис пишет структурированные JSON-логи через `log/slog`, а метрики и трейсы отправляет через OpenTelemetry. Адреса Grafana, Prometheus и Jaeger показывает `tripgoctl connect`.

Посмотреть, какие запросы дошли до Push Service:

```bash
tripgoctl environment logs push-service
```

В Prometheus и Grafana можно посмотреть, в частности:

```text
http_server_request_duration_seconds_count
http_server_active_requests
db_client_operation_duration_seconds_count
tripgo_positions_saved_total
tripgo_stale_checks_total
tripgo_position_requests_queue_size
tripgo_position_requests_workers_active
tripgo_position_requests_total
tripgo_position_requests_duration_seconds_count
```

У `tripgo_stale_checks_total` есть результаты `queued`, `duplicate` и `dropped`. Счётчик `tripgo_position_requests_total` различает `success`, `error` и `dropped`. Спан обработки задачи связан со спаном проверки через span link — ссылку между асинхронными операциями. Вызов Push Service записывается отдельным дочерним спаном обработки.

Трейсы HTTP и PostgreSQL также доступны в Jaeger. SQL-трейсер добавляет имя коллекции для известных ему таблиц; при добавлении новых таблиц его правила нужно расширять.

## Тесты и генерация

```bash
make test
make test-integration
make lint
make generate
```

`make test` запускает тесты с `-race` и `-count=2`, а также показывает покрытие домена и usecase. В тестах фоновых компонентов используется `goleak`; тики и внешние зависимости в unit-тестах управляются тестами. Для интеграционных тестов нужны запущенный PostgreSQL, актуальный `.env` и применённые миграции. Тесты создают собственные записи и удаляют их после проверки.

`make generate` обновляет OpenAPI-код и моки; сгенерированные файлы находятся в `api` и рядом с соответствующими пакетами. Для `make lint` используется `bin/golangci-lint`; в проекте применялся golangci-lint 2.15.0:

```bash
mkdir -p bin
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b ./bin v2.15.0
```

Для `make test` явно указан `GOTOOLCHAIN=go1.26.0+auto` из-за встречавшейся ошибки поиска `covdata` при автоматическом переключении версии Go.

## Остановка

По `SIGINT` или `SIGTERM` сервер прекращает приём запросов и ожидает текущие. Затем останавливается сканер, воркеры дорабатывают очередь в пределах общего `SHUTDOWN_TIMEOUT`, после чего закрываются клиент Push Service, телеметрия и пул базы.

Если общий срок истёк, ожидающие операции отменяются, ошибки закрытия записываются в лог. При этом успешную отправку оставшейся телеметрии и завершение всех действий по очистке до истечения срока гарантировать нельзя.
