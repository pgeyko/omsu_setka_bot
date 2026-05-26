# bot — Code Review Task Tracker

> Доработки по результатам `CODE_REVIEW.md` (2026-05-25).  
> Префикс `[joint]` — задачи, требующие координации с `setka/`.

---

## Этап 1 — 🔴 Критическая безопасность

> Code Review P0: Утечка API-ключа, отсутствие rate limit на auth, timing-атаки.

- [ ] **1.1** — Перенести Gemini API-ключ из URL query-параметра в HTTP-заголовок `X-Goog-Api-Key`. (`internal/llm/client.go:440`)
- [ ] **1.2** — Добавить per-IP rate limiter на эндпоинт `POST /api/auth/token`. Заменить строковое сравнение `admin_secret` на `crypto/subtle.ConstantTimeCompare`. (`internal/api/router.go:154`, `internal/api/middleware_auth.go:42`)
- [ ] **1.3** — Добавить per-IP rate limiter на эндпоинт `POST /webhook/schedule`. (`internal/api/router.go:267`)
- [ ] **1.4** — Добавить валидацию длины `admin_secret` (≤ 256) на эндпоинте `/api/auth/token`. (`internal/api/middleware_auth.go:34-43`)
- [ ] **1.5** — Установить ограничение длины `system_prompt` (≤ 10 000 символов). (`internal/api/handler_persona.go:44-46`)
- [ ] **1.6** — Сменить права доступа файлов контекста групп с 0640/0644 на 0600. (`internal/api/context_handler.go:51,89,127,215`)
- [ ] **1.7** — Убрать/обрезать логирование полного тела вебхука на debug-уровне. (`internal/handler/handler_webhook.go:101-102`)
- [ ] **1.8** — Добавить `jwt.WithValidMethods([]string{"HS256"})` в ParseWithClaims. (`internal/api/middleware_auth.go:82,120`)

---

## Этап 2 — 🔧 Инфраструктура и CI/CD

> Code Review P0/P1: Нет CI/CD, проблемы Docker, CORS.

- [ ] **2.1** — Создать `.github/workflows/ci.yml`: `golangci-lint`, `go vet`, `go test -race ./...`, `go build ./...`.
- [ ] **2.2** — Создать `.dockerignore` с исключением `node_modules/`, `.git/`, `*.md`.
- [ ] **2.3** — Вынести `go vet + go test` из Dockerfile в CI (ускорить сборку). (`Dockerfile:13`)
- [ ] **2.4** — `[joint]` Добавить `HEALTHCHECK` инструкцию в Dockerfile.
- [ ] **2.5** — Заменить `CORS_ORIGIN=*` в production конфиге на явное значение. (`config.yaml:270`)

---

## Этап 3 — 🏗️ Архитектура: рефакторинг main.go и DI

> Code Review P0: 657-строчный main(), пакетные глобалы, нет интерфейса LLM-клиента.

- [ ] **3.1** — Выделить интерфейс `LLMClient` (с методами `Call`, `CallWithHistory`). Перевести `AgentOrchestrator` с `*llm.Client` на интерфейс. (`internal/agent/orchestrator.go:25`)
- [ ] **3.2** — Выделить структуру `App` с полями для всех зависимостей. Убрать пакетные глобалы (`globalLLM`, `botUsername`, `providerCount`). (`cmd/bot/main.go:662-669`)
- [ ] **3.3** — Разбить `main()` на функции: `registerTelegramHandlers()`, `registerAPIRoutes()`, `initServices()`. (`cmd/bot/main.go:68-645`)
- [ ] **3.4** — Разбить `Server` struct (22 поля) на подструктуры: `APIConfig`, `ServiceDeps`, `BotDeps`. (`internal/api/router.go:28-50`)
- [ ] **3.5** — Заменить 13-параметровый `NewServer` на opts-паттерн или `Config` struct. (`internal/api/router.go:52-56`)
- [ ] **3.6** — Убрать ad-hoc создание `&db.DB{DB: h.db}` в хендлерах. Внедрять `*db.DB` через конструктор. (`internal/handler/handler_message.go:177`)
- [ ] **3.7** — Переименовать пакет `handlers` → `handler` (стиль Go). (`internal/handler/`)

---

## Этап 4 — ⚡ Производительность и SQLite

> Code Review P1: MaxOpenConns=10, DefaultClient без timeout, утечки памяти, N+1.

### SQLite
- [ ] **4.1** — Снизить `MaxOpenConns` с 10 до 4-6. Установить `SetConnMaxLifetime(30 * time.Minute)`. (`internal/db/db.go:34-36`)

### Утечки ресурсов
- [ ] **4.2** — Создать переиспользуемый HTTP-клиент с 30s timeout вместо `http.DefaultClient`. (`internal/handler/handler_message.go:377`)
- [ ] **4.3** — Исправить `mediaGroupMessages` cleanup: удалять только записи старше 30 минут, а не все. (`cmd/bot/main.go:296-299`)
- [ ] **4.4** — Добавить TTL-очистку в `UsernameCache` (удалять записи старше 24ч). (`internal/telegram/username_cache.go`)

### N+1 и аллокации
- [ ] **4.5** — Переписать `fuzzySlugMatch` на SQL `LIKE` вместо загрузки всех топиков. (`internal/handler/handler_message.go:408-410`)
- [ ] **4.6** — Заменить ручную конкатенацию в `stringsJoin` на `strings.Join`. (`internal/agent/orchestrator.go:371-380`)
- [ ] **4.7** — Заменить `+=` в `fillPrompt` на `strings.Builder`. (`internal/classifier/classifier.go:137-147`)

### Request ID
- [ ] **4.8** — Добавить middleware с генерацией request ID + `slog.With`. (`internal/api/router.go`)

---

## Этап 5 — 🔗 Интеграционный контракт setka ↔ bot

> Code Review P1: Webhook registration URL без порта, необработанные change fields.

- [ ] **5.1** — Исправить fallback URL в `RegisterWebhooksWithSetka`: добавить порт `:8081`. (`internal/telegram/sync.go:42-43`)
- [ ] **5.2** — Добавить классификацию аномалий для полей `teacher`, `pair`, `date`, `subgroup` в diff-движке. (`internal/schedule/diff.go:159-207`)
- [ ] **5.3** — Унифицировать регистрацию вебхуков: один HTTP-метод (PUT `/api/v1/admin/webhooks/by-url`), убрать дублирование POST/PUT. (`internal/telegram/sync.go` vs `cmd/bot/helpers.go`)

---

## Этап 6 — 🧪 Тесты и надёжность

> Code Review: AgentOrchestrator, WebhookHandler, AuthMiddleware — без тестов.

### Критические тесты
- [ ] **6.1** — Выделить `LLMClient` interface → написать mockLLMClient → покрыть `AgentOrchestrator` table-driven тестами (tool filtering, max steps, async dispatch, error paths). (`internal/agent/orchestrator_test.go`)
- [ ] **6.2** — Написать тесты для `WebhookHandler`: HMAC verification (valid/invalid/malformed), timestamp skew (±5 min boundary), event dedup (new/duplicate/expired), chatID resolution. (`internal/handler/handler_webhook_test.go`)
- [ ] **6.3** — Написать тесты для AuthMiddleware: JWT generation, parsing, expiration, revocation, role enforcement. (`internal/api/middleware_auth_test.go`)
- [ ] **6.4** — Добавить `t.Parallel()` во все тесты с изолированными БД.

### Интеграционные тесты (joint)
- [ ] **6.5** `[joint]` — Написать end-to-end тест для вебхук-границы: setka → bot (payload → HMAC → dedup → diff engine → announcement).
- [ ] **6.6** `[joint]` — Написать тест для регистрации подписчика: bot POST → setka upsert → GET verify.

### Race detector
- [ ] **6.7** — Проверить все новые тесты race detector'ом.

---

## Этап 7 — 🧹 Технический долг

> Code Review P2: React SPA рефакторинг, логгирование, мелкие улучшения.

### React SPA (admin/)
- [ ] **7.1** — Разбить `GroupsPage.tsx` (551 строка) на подкомпоненты: GroupList, GroupDetail, GroupSettings. (`admin/src/pages/GroupsPage.tsx`)
- [ ] **7.2** — Объединить `request()` и `requestFull()` в единый метод. (`admin/src/api/client.ts`)
- [ ] **7.3** — Внедрить Feature-Sliced Design: создать директории `entities/`, `features/`, `shared/`. Перенести типы и хуки.
- [ ] **7.4** — Вынести Nav-конфигурацию из Layout в отдельный файл `constants/navigation.ts`. (`admin/src/components/Layout.tsx`)
- [ ] **7.5** — Убрать инлайн `document.documentElement.setAttribute('data-theme', ...)` из Layout. (`admin/src/components/Layout.tsx:27`)
- [ ] **7.6** — Выделить хуки для API-запросов в `hooks/useGroups.ts`, `hooks/useTopics.ts` и т.д.

### Логгирование
- [ ] **7.7** — Исправить debug-логирование аргументов тулов: логировать только имена инструментов, не полные arguments. (`internal/agent/tools.go:208`)
- [ ] **7.8** — Обрезать/суммировать логирование LLM-истории на debug-уровне. (`internal/llm/client.go:228`)

### Мелкие улучшения
- [ ] **7.9** — Кэшировать результат `GetChatAdministrators` для всех админов из одного ответа, а не по одному user. (`internal/telegram/admin_cache.go:49-51`)
- [ ] **7.10** — Переименовать пакет `handlers` → `handler` (стиль Go). (`internal/handler/`)

---

## Легенда

```
[ ]  не начато
[/]  в процессе
[x]  выполнено
[!]  заблокировано
```
