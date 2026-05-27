# omsu_bot — Task Tracker

> **Правила работы с этим файлом** (обязательны для агентов):
>
> - Перед началом задачи: пометить `[/]`
> - После завершения задачи: пометить `[x]` + однострочная заметка
> - После завершения **всего этапа**: добавить `✅ Completed: YYYY-MM-DD`
> - Не переходить к следующему этапу пока все задачи текущего не `[x]`
> - Заблокированные задачи: `[!]` + описание блокера

---

## Этап 0 — Доработки в omsu_setka ✅ Completed: 2026-05-20

> Необходимо для работы webhook-уведомлений. Работа велась в отдельном репозитории [omsu_setka](https://github.com/pgeyko/omsu_setka).

- [x] Добавить таблицу `webhook_subscribers` в storage
- [x] Создать `webhook_repo.go` (POST + HMAC-SHA256)
- [x] Создать `notifier.go` (POST + HMAC-SHA256)
- [x] Добавить `group_id` в payload изменений
- [x] Подключить `WebhookNotifier` в syncer
- [x] Добавить Admin API: `POST/GET/DELETE /api/v1/admin/webhooks`
- [x] Добавить `WEBHOOK_*` переменные в конфиг
- [x] Написать тесты

---

## Этап 1 — Фундамент ✅ Completed: 2026-05-20

> Skills: `golang-pro`, `sql-pro`, `lint-and-validate`, `writing-plans`

- [x] Инициализировать Go-модуль: `go mod init omsu_bot`
- [x] Структура директорий по схеме из `AGENTS.md`
- [x] `internal/config/config.go` — cleanenv конфиг (env + YAML)
- [x] `config.yaml` — шаблон конфигурации
- [x] `.env.example` — все env-переменные с описаниями
- [x] `internal/db/db.go` — открытие SQLite, WAL mode, busy_timeout
- [x] `internal/db/migrate.go` — миграция: все 8 таблиц
- [x] Seed-загрузка `bot_persona` из `persona.md` при пустой таблице
- [x] `internal/persona/persona.go` — Store с методами Load/Get/Update/Reset
- [x] `internal/persona/seed_parser.go` — парсинг секций из `persona.md`
- [x] `persona.md` — дефолтный seed-файл личности
- [x] `internal/llm/provider.go` — структуры Provider, Chain, capabilities
- [x] `internal/llm/client.go` — HTTP клиент Gemini/DeepSeek, `buildMessages()` с persona
- [x] `internal/llm/tracker.go` — учёт токенов, дневной лимит, алерты
- [x] `internal/llm/prompts.go` — загрузка промптов из `prompts/*.txt`
- [x] `prompts/classify.txt`
- [x] `prompts/forward_intent.txt` (заменён агентным подходом в Этапе 8)
- [x] `prompts/topic_command.txt` (заменён агентным подходом в Этапе 8)
- [x] `prompts/schedule_announce.txt`
- [x] `cmd/bot/main.go` — инициализация всех зависимостей, graceful shutdown
- [x] Базовый Telegram-бот: longpolling, middleware rate-limit (5 req/min/user) — реализовано в Этапе 2 (longpolling) + Этапе 15 P1#6 (middleware)
- [x] `go build ./...`, `go vet ./...` — чисто

---

## Этап 2 — Пересылка сообщений ✅ Completed: 2026-05-22

> Skills: `golang-pro`, `api-endpoint-builder`, `lint-and-validate`

- [x] `internal/handler/handler_message.go` — роутер входящих сообщений
- [x] Предфильтр: длина ≥ 15 слов ИЛИ есть вложение
- [x] Дедупликация через `processed_messages` (проверка до LLM-вызова)
- [x] `internal/classifier/classifier.go` — LLM классификация текста
- [x] Vision: resize фото до 512px → Gemini vision (если провайдер multimodal) — `resizeIfNeeded()` + `HasMultimodalProvider()`
- [x] Fallback без vision если провайдер не multimodal — возвращаем `("", nil)`, caller продолжает как text-only
- [x] Кэш классификации по `file_id` (повторное фото не отправлять в LLM)
- [x] Circuit breaker: 3 ошибки подряд → disabled 5 мин (в `llm/provider.go`)
- [x] `internal/forwarder/forwarder.go` — `copyMessage` + шапка с `persona.name`
- [x] Ответ в исходный топик со ссылкой `↗️ Продублировал в «Топик» → [ссылка]`
- [x] `internal/handler/handler_mention.go` — обработка `@bot` команд
- [x] LLM парсинг интента `forward_intent` (заменён на AgentOrchestrator в Этапе 8)
- [x] Нечёткое совпадение топика по `name` + `aliases`
- [x] Переспрос при нераспознанном топике
- [x] `go build ./...`, `go vet ./...` — чисто

---

## Этап 3 — Интеграция расписания ✅ Completed: 2026-05-20

> Skills: `golang-pro`, `api-endpoint-builder`, `lint-and-validate`

- [x] `internal/handler/handler_webhook.go` — `POST /webhook/schedule`
- [x] HMAC-SHA256 валидация заголовка `X-Webhook-Signature`
- [x] Десериализация payload: `type`, `group_id`, `changes[]`
- [x] `internal/schedule/diff.go` — детектор аномалий + сохранение
- [x] Детектор аномалий: ANOMALY_BUILDING, ANOMALY_ROOM, ANOMALY_SUBJECT, ANOMALY_CANCEL
- [x] Сохранение снапшота в `schedule_snapshots`
- [x] Сохранение аномалий в `schedule_anomalies`
- [x] `internal/schedule/announcer.go` — генерация объявления через LLM (в Этапе 14 интегрирован в DiffEngine с fallback на Go-форматтер)
- [x] Пост в `announce_thread_id` с аномальными пометками ⚠️
- [x] Регистрация бота в omsu_setka: `POST /api/v1/admin/webhooks` при старте
- [x] `go build ./...`, `go vet ./...` — чисто

---

## Этап 4 — Admin REST API ✅ Completed: 2026-05-20

> Skills: `golang-pro`, `api-endpoint-builder`, `api-documentation`, `lint-and-validate`

- [x] `internal/api/router.go` — Fiber app, CORS, recover middleware
- [x] `internal/api/middleware_auth.go` — JWT HS256 (`POST /api/auth/token`)
- [x] `internal/api/handler_persona.go`
  - [x] `GET  /api/persona` — текущие настройки
  - [x] `PUT  /api/persona` — обновить name/system_prompt/signature
  - [x] `POST /api/persona/reset` — сбросить к persona.md
- [x] `internal/api/handler_topics.go`
  - [x] `GET    /api/topics`
  - [x] `POST   /api/topics`
  - [x] `GET    /api/topics/:id`
  - [x] `PUT    /api/topics/:id`
  - [x] `DELETE /api/topics/:id`
  - [x] `POST   /api/topics/:id/close`
  - [x] `POST   /api/topics/:id/open`
- [x] `internal/api/handler_permissions.go`
  - [x] `GET /api/permissions`
  - [x] `PUT /api/permissions/:command`
- [x] `internal/api/handler_stats.go`
  - [x] `GET /api/stats/tokens`
  - [x] `GET /api/stats/requests`
  - [x] `GET /api/stats/forwards`
  - [x] `GET /api/stats/messages`
  - [x] `GET /api/stats/providers`
- [x] `GET /api/schedule/snapshots`
- [x] `GET /api/schedule/anomalies`
- [x] `go build ./...`, `go vet ./...` — чисто

---

## Этап 5 — CRUD топиков через бота ✅ Completed: 2026-05-20

> Skills: `golang-pro`, `lint-and-validate`

- [x] LLM парсинг команд: создать / закрыть / переименовать топик
- [x] Проверка прав (`topic_crud` → только admin) через `getChatAdministrators` + кэш 10 мин
- [x] `createForumTopic` → запись в `topics` (синхронно)
- [x] `editForumTopic` → обновление `topics`
- [x] `closeForumTopic` / `reopenForumTopic`
- [x] Обработка ошибок Telegram API
- [x] `go build ./...`, `go vet ./...` — чисто

---

## Этап 6 — Саммари ✅ Completed: 2026-05-20

> Skills: `golang-pro`, `lint-and-validate`

- [x] Кольцевой буфер 200 сообщений на топик (in-memory)
- [x] `@bot что пропустил?` / `@bot саммари` — trigger
- [x] Rate limit: 1 запрос / 30 мин / пользователь (`summary_requests` таблица)
- [x] LLM-генерация саммари из буфера
- [x] Пост реплаем в тот же топик
- [x] Настройка прав через `command_permissions`
- [x] `go build ./...`, `go vet ./...` — чисто

---

## Этап 7 — Деплой и финализация ✅ Completed: 2026-05-20

> Skills: `docker-expert`, `bash-pro`, `commit`, `lint-and-validate`

- [x] `Dockerfile` — multi-stage build, alpine, non-root user
- [x] `Dockerfile.dev` — hot-reload через air
- [x] `docker-compose.yml` — бот + volume для SQLite
- [x] `.env.example` — финальная версия со всеми переменными
- [x] `README.md` — инструкция по деплою для новой группы
- [x] Git tag `v1.0.0` (по готовности)

## Этап 8 — Мультиарендность, Управление Контекстом и Агентный подход ✅ Completed: 2026-05-21

- [x] Миграция SQLite на поддержку многих групп (groups, superadmins)
- [x] Реализация загрузки контекстов (persona.md, system_prompt.txt, knowledge_base.txt, features.json) через API
- [x] Разработка ИИ-агента (Agent Loop / Orchestrator) с поддержкой динамических инструментов (Tools)
- [x] Интеграция UsernameCache в обработку сообщений
- [x] Реализация Antispam-компонента (Flood Control, Link Filter, Captcha с inline кнопками)
- [x] Расширение client.go и создание MediaProcessor для обработки Vision OCR и Voice STT в Gemini
- [x] Сборка и интеграция в main.go, успешный запуск тестов

---

## Этап 9 — Интерактивная админка в Telegram, API суперадмина и группы без топиков ✅ Completed: 2026-05-21

- [x] Создание state-менеджера SessionStore для обработки пошагового ввода администратора
- [x] Реализация в-боте хендлера команды `/settings` и CallbackQuery переходов меню настроек
- [x] Разработка логики пошагового ввода настроек: сохранение KB, личности, промпта
- [x] Интеграция API поиска Setka: запрос, вывод списка кнопок, сохранение `omsu_group_id`
- [x] Добавление REST API эндпоинтов для суперадминов и синхронизации вебхуков
- [x] Оптимизация работы в группах без топиков: детекция и обход классификации
- [x] Проверка и финальное тестирование (юнит-тесты и ручная валидация)

---

## Этап 10 — Локальная модерация по группам, REST API и Swagger ✅
✅ Completed: 2026-05-22

- [x] Создать и настроить defaultFeatures() хелпер в Go-бэкенде
- [x] Добавить кнопки управления локальной модерацией в меню настроек Telegram
- [x] Refactor antispam.go для применения per-group toggles
- [x] Обновить featuresForm в React Admin UI
- [x] Отрендерить красивое разделение на "Основные модули" и "Локальная модерация" в админ панели
- [x] Добавить swagger stubs для всех эндпоинтов в swagger_docs.go
- [x] Сгенерировать Swagger спецификации и проверить тесты

---

## Этап 11 — Аудит безопасности и стабилизация ✅
✅ Completed: 2026-05-22

- [x] Permission check для agent tools (moderate_user/run_protocol/manage_topic)
- [x] CopyMessage получил MessageThreadID — копия в правильный топик
- [x] Tracker восстанавливает дневной лимит токенов из БД при старте
- [x] Бэкап БД перед destructive migration
- [x] GlobalVoiceTranscription/PhotoProcessing → atomic.Bool (data race fix)
- [x] joinedUsers cleanup в antispam
- [x] SessionStore TTL (30 мин) + StartCleanup
- [x] Provider.state защищён sync.Mutex
- [x] registerWithSetka использует cfg.Setka.PublicURL вместо localhost
- [x] DB cleanup goroutine + VACUUM раз в неделю
- [x] HTTP timeouts (15-30s) во всех 6 исходящих вызовах
- [x] makeSlug/resolveDate вынесены в internal/util (bugfix weekday=today)
- [x] visionCache LRU (200 записей, TTL 1ч)
- [x] protocols.json кэшируется через sync.Once

---

## Этап 12 — Хэштеги, настройки, рефакторинг ✅
✅ Completed: 2026-05-22

- [x] message_tags таблица + /tag команда
- [x] Дедупликация по тексту (substr 100 символов)
- [x] Три режима обработки фото (off/auto/@mention)
- [x] ReplyParameters в ответах агента
- [x] Dedicated /settings /настройки handlers (BotCommand entity fix)
- [x] Captcha callback: defer AnswerCallbackQuery, wrong-user alert
- [x] Три provider chain: text (gemma), vision (gemma→3.1-flash), audio (3.1-flash)
- [x] Per-provider rate limiter (RPM/TPM/RPD)
- [x] Cleanup dead code (+711 строк)
- [x] Remove hardcoded keywords + forced instructions from mention handler

---

## Этап 13 — Промпты, персона, документация ✅
✅ Completed: 2026-05-23

- [x] Extract LLM prompts from main.go to prompts/ (greeting, status_report)
- [x] Extract bot messages to messages.yaml (internal/messages)
- [x] Remove all hardcoded strings from cmd/bot/main.go
- [x] Update persona.md: comprehensive Russian synonym mapping
- [x] Update tool descriptions with Russian aliases (замути/забань/тема etc.)
- [x] /init omsu_id optional (can init without Setka ID)
- [x] Filter restricted tools pre-LLM for non-admin users
- [x] Owner recognized as admin in orchestrator
- [x] /summary defaults to current topic
- [x] setMyCommands API at startup (Telegram command menu)
- [x] get_schedule tool appends Setka public URL
- [x] README.md created

---

## Этап 14 — Агентный подход: доработки и унификация промптов
✅ Completed: 2026-05-23

- [x] Добавить инструмент `forward_message` в AgentOrchestrator/ ToolExecutor (пересылка через агента)
- [x] Переписать `persona.md`: явные инструкции для tool calling, триггеры, словарь синонимов
- [x] Вынести OCR/STT промты из `media_processor.go` в `prompts/ocr.txt` и `prompts/stt.txt`
- [x] Переместить `helpText` из `main.go` в `messages.yaml` (поле `help_commands`)
- [x] Внедрить имена протоколов из `protocols.json` в описание инструмента `run_protocol`
- [x] Интегрировать `Announcer` (LLM-генерация объявлений) в `DiffEngine.ProcessWebhook()`
- [x] Исправить архитектуру в `AGENTS.md` (`internal/bot/` → `internal/handler/`)
- [x] Обновить схему БД в `AGENTS.md` (добавить мультиарендные таблицы)
- [x] Добавить примечание о мультиарендности в `GROUPBOT_TZ.md`
- [x] Удалить упоминания несуществующих промтов `forward_intent.txt`/`topic_command.txt`
- [x] Удалить `internal/api/persona.md` с неверным форматом
- [x] Вынести шаблон `systemExtra` в `prompts/agent_context.txt`
- [x] Унифицировать стиль всех промтов (единая структура)
- [x] Добавить защиту от prompt injection в `persona.md`

---

---

## Этап 15 — Аудит 2026-05-24: безопасность и связность ✅
✅ Completed: 2026-05-24

> Исправления по результатам AUDIT_2026-05-24.md (кроме P0#1 — ротация ключей, тестовые).

- [x] P0#2 Multi-tenancy вебхука: resolve chat_id из omsu_group_id
- [x] P0#3 Idempotent регистрация: upsert по URL в Setka + PUT /webhooks/by-url
- [x] P0#4 Entity type в payload: validate entity_type == "group"
- [x] P0#5 nil-safe msg.From: helper util.MessageSender() во всех хендлерах
- [x] P1#6 Telegram rate-limit: middleware подключен к active groups handler
- [x] P1#7 command_permissions: permissions.Service + интеграция в orchestrator
- [x] P1#9 Prompt path traversal: regex + filepath.Clean + HasPrefix
- [x] P1#10 BodyLimit: 512 KB глобально, соответствие контекстным роутам
- [x] P1#11 Webhook replay protection: timestamp + event_id + dedup (webhook_events table)
- [x] P1#12 Setka admin API: PUT /webhooks/by-url + PATCH /webhooks/:id
- [x] P2#13 Schedule diff: hasNewLessons() — новые пары тоже объявляются

---

## Этап 16 — Аудит безопасности 2026-05-26: P0/P1 фиксы
✅ Completed: 2026-05-26

> Skills: `golang-pro`, `lint-and-validate`

- [x] P0#1 Gemini API key: URL query param → `x-goog-api-key` header (`llm/client.go`)
- [x] P0#2 Auth rate-limit: 5 req/min/IP на `/api/auth/token` (`middleware_auth.go` + `router.go`)
- [x] P0#3 JWT algorithm confusion: `jwt.WithValidMethods(["HS256"])` в ParseWithClaims (`middleware_auth.go`)
- [x] P0#4 Constant-time admin_secret сравнение через `subtle.ConstantTimeCompare` (`middleware_auth.go`)
- [x] P1#5 Admin_secret min length check (16 символов) (`middleware_auth.go`)
- [x] P1#6 Uniform file permissions: features.json 0644→0640, dir 0755→0750 (`context_handler.go`)
- [x] P1#7 system_prompt length validation (≤10000 символов) (`handler_persona.go`)
- [x] P1#8 Webhook debug log: truncated body to 500 chars + body_size field (`handler_webhook.go`)
- [x] P1#9 Dockerfile: removed `go test` from build stage, added HEALTHCHECK
- [x] P1#10 Created `.dockerignore` (19 entries)

---

## Этап 17 — Архитектурный рефакторинг
✅ Completed: 2026-05-26

> Skills: `golang-pro`, `lint-and-validate`

- [x] 2.1 LLMClient interface extracted to `llm/client.go` (6 методов, 5 потребителей обновлены)
- [x] 2.2 App struct created, 6 package globals consolidated into `var app *App`
- [x] 2.3 `initLLM()` extracted from main() (провайдеры → цепи → клиент)
- [x] 2.4+2.5 `ServerConfig` struct created, `NewServer` 20→9 positional args
- [x] 2.6 `http.DefaultClient` → `http.Client{Timeout: 30s}` в classifyPhoto

---

## Этап 18 — Производительность
✅ Completed: 2026-05-26

> Skills: `golang-pro`, `lint-and-validate`, `performance-optimizer`

- [x] 3.1 MaxOpenConns 10→4, MaxIdleConns 10→4 (`db/db.go`)
- [x] 3.2 Reusable http client: moved to `Handler.httpClient` field, initialized once in `NewHandler` (`handler_message.go`)
- [x] 3.3 fuzzySlugMatch: SQL LIKE pre-filter to reduce rows fetched from SQLite (`handler_message.go`)
- [x] 3.4 classifier.fillPrompt: `topicStr +=` → `strings.Builder` (`classifier/classifier.go`)
- [x] 3.5 Media group cleanup TTL: extracted constants `processedMediaGroupsTTL` (10m) + `mediaGroupMessagesTTL` (30m) (`cmd/bot/main.go`)
- [x] 3.6 Username cache: added TTL-based eviction (6h TTL, 30m cleanup interval) (`telegram/username_cache.go`)
- [x] 3.7 Request ID middleware: `X-Request-ID` header + `c.Locals("request_id", ...)` + log injection (`api/router.go`)

---

## Этап 19 — Интеграционный контракт setka ↔ bot
✅ Completed: 2026-05-26

> Skills: `golang-pro`, `lint-and-validate`, `api-endpoint-builder`

- [x] 5.1 Fallback URL порт :8081 в RegisterWebhooksWithSetka (`telegram/sync.go`)
- [x] 5.2 Добавлены AnomalyTeacher, AnomalyPair, AnomalyDate, AnomalySubgroup в classifyChange + buildAnnouncement (`schedule/diff.go`)
- [x] 5.3 Унификация: registerWithSetka переведён с POST на PUT /api/v1/admin/webhooks/by-url (`cmd/bot/helpers.go`)

---

## Этап 20 — Технический долг
✅ Completed: 2026-05-26

> Skills: `golang-pro`, `lint-and-validate`, `web-design-guidelines`

### Go backend
- [x] 7.7 Убраны `arguments` из debug-лога тула (`agent/tools.go`)
- [x] 7.8 `resp.Content` в llm response log → `util.Truncate(..., 500)` (`llm/client.go`)
- [x] 7.9 `IsOwner` использует кеш админов; `adminCacheEntry` хранит `isOwner` (`telegram/admin_cache.go`)
- [x] 7.10 `package handlers` → `package handler` (11 файлов + 2 потребителя)

### React SPA (admin/)
- [x] 7.1 GroupsPage (551→78 строк) разбит на GroupList, GroupDetail, GroupSettings, CreateGroupModal (`entities/groups/`)
- [x] 7.2 `request` + `requestFull` объединены в единый `request()`, возвращающий `ApiResponse<T>` (`shared/api/client.ts`)
- [x] 7.3 FSD: созданы `shared/` (api, stores, ui, constants, hooks), `entities/groups/`, `features/`
- [x] 7.4 Nav-конфигурация вынесена в `shared/constants/navigation.ts`
- [x] 7.5 `document.documentElement.setAttribute` → `useTheme()` хук (`shared/hooks/useTheme.ts`)
- [x] 7.6 API-хуки выделены в `entities/groups/hooks.ts`

---

## Этап 21 — Тесты и надёжность
✅ Completed: 2026-05-26

> Skills: `golang-pro`, `lint-and-validate`

- [x] 6.1 `mockLLMClient` + `ToolExecutorInterface` + 7 table-driven тестов для `AgentOrchestrator` (tool filtering, max steps, async dispatch, error paths, sequential) (`agent/orchestrator_test.go`)
- [x] 6.2 10 тестов для `WebhookHandler`: HMAC valid/invalid/missing, timestamp skew ±5m, event dedup new/duplicate/expired, chatID resolution found/not-found (`handler/handler_webhook_test.go`)
- [x] 6.3 8 тестов для `AuthMiddleware`: JWT generation, parsing, expiration, revocation, role enforcement, invalid body (`api/middleware_auth_test.go`)
- [x] 6.4 `t.Parallel()` добавлен во все DB-backed тесты (56+ test functions в 10+ файлах)
- [x] 6.7 Race detector: `go test -race` проходит на всех ключевых пакетах (agent, api, handler, llm, telegram)

## Этап 22 — Core refactoring
✅ Completed: 2026-05-27

> Ветка: `feat/core-refactor`
> Skills: `golang-pro`, `lint-and-validate`

- [x] Repository layer: topics_repo (10 методов), processed_repo (3 метода), media_repo (3 метода)
- [x] 11 table-driven тестов репозиториев (topics, processed, media)
- [x] handler_message.go: SQL-запросы заменены на вызовы репозиториев (-35 строк)
- [x] ToolExecutor: switch → map[string]ToolFunc, 9 позиционных аргументов → ToolDeps
- [x] CommandHandler: реестр команд с init()-регистрацией
- [x] slash.go (340→61 строка): dispatch через CommandHandler registry
- [x] telegramPoster → schedule.BotPoster (вынесен из main.go)
- [x] Глобальный `var app` удалён, заменён на package-level vars
- [x] initPersona, startSighupHandler, setupLogger → internal/app/
- [x] Мёртвый код: App struct, BotPosterAdapter, dispatchSlashCommand удалены
- [x] docs: AGENTS.md и ARHITEKTURA.md синхронизированы

## Легенда

```
[ ]  не начато
[/]  в процессе
[x]  выполнено
[!]  заблокировано
```

