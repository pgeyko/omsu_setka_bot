# omsu_bot — Task Tracker

> **Правила работы с этим файлом** (обязательны для агентов):
>
> - Перед началом задачи: пометить `[/]`
> - После завершения задачи: пометить `[x]` + однострочная заметка
> - После завершения **всего этапа**: добавить `✅ Completed: YYYY-MM-DD`
> - Не переходить к следующему этапу пока все задачи текущего не `[x]`
> - Заблокированные задачи: `[!]` + описание блокера

---

## Этап 0 — Доработки в omsu_mirror (параллельно с ботом)

> Необходимо для работы webhook-уведомлений. Работа ведётся в `omsu_mirror/`.

- [ ] Добавить таблицу `webhook_subscribers` в `omsu_mirror/core/internal/storage/sqlite.go`
- [ ] Создать `omsu_mirror/core/internal/storage/webhook_repo.go`
- [ ] Создать `omsu_mirror/core/internal/webhook/notifier.go` (POST + HMAC-SHA256)
- [ ] Добавить `group_id` в payload `compareAndLogChanges` → `notifier.Notify()`
- [ ] Подключить `WebhookNotifier` в `omsu_mirror/core/internal/sync/syncer.go`
- [ ] Добавить Admin API: `POST/GET/DELETE /api/v1/admin/webhooks` в omsu_mirror
- [ ] Добавить `WEBHOOK_*` переменные в `omsu_mirror/core/internal/config/config.go`
- [ ] Написать тесты: `webhook_repo_test.go`, `notifier_test.go`

---

## Этап 1 — Фундамент

> Skills: `golang-pro`, `sql-pro`, `lint-and-validate`, `writing-plans`

- [ ] Инициализировать Go-модуль: `go mod init omsu_bot`
- [ ] Структура директорий по схеме из `AGENTS.md`
- [ ] `internal/config/config.go` — cleanenv конфиг (env + YAML)
- [ ] `config.yaml` — шаблон конфигурации
- [ ] `.env.example` — все env-переменные с описаниями
- [ ] `internal/db/db.go` — открытие SQLite, WAL mode, busy_timeout
- [ ] `internal/db/migrations/0001_init.sql` — все таблицы включая `bot_persona`
- [ ] Seed-загрузка `bot_persona` из `persona.md` при пустой таблице
- [ ] `internal/persona/persona.go` — Store с методами Load/Get/Update/Reset
- [ ] `internal/persona/seed_parser.go` — парсинг секций из `persona.md`
- [ ] `persona.md` — дефолтный seed-файл личности
- [ ] `internal/llm/provider.go` — структуры Provider, Chain, capabilities
- [ ] `internal/llm/client.go` — HTTP клиент Gemini/DeepSeek, `buildMessages()` с persona
- [ ] `internal/llm/tracker.go` — учёт токенов, дневной лимит, алерты
- [ ] `internal/llm/prompts.go` — загрузка промптов из `prompts/*.txt`
- [ ] `prompts/classify.txt`
- [ ] `prompts/forward_intent.txt`
- [ ] `prompts/topic_command.txt`
- [ ] `prompts/schedule_announce.txt`
- [ ] `cmd/bot/main.go` — инициализация всех зависимостей, graceful shutdown
- [ ] Базовый Telegram-бот: longpolling, middleware rate-limit (5 req/min/user)
- [ ] `go test ./...` — все тесты зелёные

---

## Этап 2 — Пересылка сообщений

> Skills: `golang-pro`, `api-endpoint-builder`, `lint-and-validate`

- [ ] `internal/bot/handler_message.go` — роутер входящих сообщений
- [ ] Предфильтр: длина ≥ 15 слов ИЛИ есть вложение
- [ ] Дедупликация через `processed_messages` (проверка до LLM-вызова)
- [ ] `internal/classifier/classifier.go` — LLM классификация текста
- [ ] Vision: resize фото до 512px → Gemini vision (если провайдер multimodal)
- [ ] Fallback без vision если провайдер не multimodal
- [ ] Кэш классификации по `file_id` (повторное фото не отправлять в LLM)
- [ ] Circuit breaker: 3 ошибки подряд → disabled 5 мин
- [ ] `internal/forwarder/forwarder.go` — `copyMessage` + шапка с `persona.name`
- [ ] Ответ в исходный топик со ссылкой `↗️ Продублировал в «Топик» → [ссылка]`
- [ ] `internal/bot/handler_mention.go` — обработка `@bot` команд
- [ ] LLM парсинг интента `forward_intent`
- [ ] Нечёткое совпадение топика по `name` + `aliases`
- [ ] Переспрос при нераспознанном топике
- [ ] `go test ./...` — все тесты зелёные

---

## Этап 3 — Интеграция расписания

> Skills: `golang-pro`, `api-endpoint-builder`, `lint-and-validate`

- [ ] `internal/bot/handler_webhook.go` — `POST /webhook/schedule`
- [ ] HMAC-SHA256 валидация заголовка `X-Webhook-Signature`
- [ ] Десериализация payload: `type`, `group_id`, `changes[]`
- [ ] `internal/schedule/diff.go` — вычисление diff без LLM
- [ ] Детектор аномалий: ANOMALY_BUILDING, ANOMALY_ROOM, ANOMALY_SUBJECT, ANOMALY_CANCEL
- [ ] Сохранение снапшота в `schedule_snapshots`
- [ ] Сохранение аномалий в `schedule_anomalies`
- [ ] `internal/schedule/announcer.go` — LLM-генерация человеческого объявления
- [ ] Пост в `announce_thread_id` (из config) с аномальными пометками ⚠️
- [ ] Регистрация бота в omsu_mirror: `POST /api/v1/admin/webhooks` при старте
- [ ] `go test ./...` — все тесты зелёные

---

## Этап 4 — Admin REST API

> Skills: `golang-pro`, `api-endpoint-builder`, `api-documentation`, `lint-and-validate`

- [ ] `internal/api/router.go` — Fiber app, CORS, recover middleware
- [ ] `internal/api/middleware_auth.go` — JWT HS256 (`POST /api/auth/token`)
- [ ] `internal/api/handler_persona.go`
  - [ ] `GET  /api/persona` — текущие настройки
  - [ ] `PUT  /api/persona` — обновить name/system_prompt/signature
  - [ ] `POST /api/persona/reset` — сбросить к persona.md
- [ ] `internal/api/handler_topics.go`
  - [ ] `GET    /api/topics`
  - [ ] `POST   /api/topics` (+ createForumTopic в Telegram)
  - [ ] `GET    /api/topics/:id`
  - [ ] `PUT    /api/topics/:id`
  - [ ] `DELETE /api/topics/:id` (+ deleteForumTopic в Telegram)
  - [ ] `POST   /api/topics/:id/close`
  - [ ] `POST   /api/topics/:id/open`
- [ ] `internal/api/handler_permissions.go`
  - [ ] `GET /api/permissions`
  - [ ] `PUT /api/permissions/:command`
- [ ] `internal/api/handler_stats.go`
  - [ ] `GET /api/stats/tokens`
  - [ ] `GET /api/stats/requests`
  - [ ] `GET /api/stats/forwards`
  - [ ] `GET /api/stats/messages`
  - [ ] `GET /api/stats/providers`
- [ ] `GET /api/schedule/snapshots`
- [ ] `GET /api/schedule/anomalies`
- [ ] `go test ./...` — все тесты зелёные

---

## Этап 5 — CRUD топиков через бота

> Skills: `golang-pro`, `lint-and-validate`

- [ ] LLM парсинг команд: создать / закрыть / переименовать топик
- [ ] Проверка прав (`topic_crud` → только admin)
- [ ] `createForumTopic` → запись в `topics` (синхронно)
- [ ] `editForumTopic` → обновление `topics`
- [ ] `closeForumTopic` / `reopenForumTopic`
- [ ] Обработка ошибок Telegram API (права, лимиты)
- [ ] `go test ./...` — все тесты зелёные

---

## Этап 6 — Саммари (после стабилизации)

> Skills: `golang-pro`, `lint-and-validate`
> Реализовать только после того, как этапы 1–5 стабильно работают в prod.

- [ ] Кольцевой буфер 200 сообщений на топик (in-memory)
- [ ] `@bot что пропустил?` / `@bot саммари` — trigger
- [ ] Rate limit: 1 запрос / 30 мин / пользователь (`summary_requests` таблица)
- [ ] LLM-генерация саммари из буфера
- [ ] Пост реплаем в тот же топик
- [ ] Настройка прав через `command_permissions`
- [ ] `go test ./...` — все тесты зелёные

---

## Этап 7 — Деплой и финализация

> Skills: `docker-expert`, `bash-pro`, `commit`, `lint-and-validate`

- [ ] `Dockerfile` — multi-stage build, alpine, non-root user
- [ ] `docker-compose.yml` — бот + volume для SQLite
- [ ] `.env.example` — финальная версия со всеми переменными
- [ ] `README.md` — инструкция по деплою для новой группы
- [ ] Smoke-тест: проверить webhook end-to-end с omsu_mirror
- [ ] Smoke-тест: обновить persona через API → проверить изменение стиля ответов
- [ ] Git tag `v1.0.0`

---

## Легенда

```
[ ]  не начато
[/]  в процессе
[x]  выполнено
[!]  заблокировано
```
