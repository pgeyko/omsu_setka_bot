# Архитектура GroupBot

> Полное описание логики работы, паттернов и потока данных.

---

## 1. Общая архитектура

```
Telegram (супергруппа с топиками)
    │
    ▼
┌─────────────────────┐     ┌──────────────────┐
│  Telegram Bot API   │◄────│  groupbot         │
│  (long polling)     │     │  (один процесс)   │
└─────────────────────┘     │                  │
                            │  ┌────────────┐  │
omsu_setka                  │  │  Fiber API  │──┼──→ Admin UI / Swagger
    │                       │  │  :8081      │  │
    └──POST /webhook/schedule─▶│  └────────────┘  │
       (HMAC-SHA256)         │                  │
                            │  ┌────────────┐  │
                            │  │  SQLite    │  │
                            │  └────────────┘  │
                            └──────────────────┘
```

**Принципы:**
- Мультиарендный — один процесс обслуживает несколько групп
- Всё состояние в SQLite (кроме LLM circuit breaker)
- Telegram API — long polling (не webhook)
- HTTP-сервер (Fiber) для админки и вебхуков от setka

---

## 2. Поток сообщений

### 2.1 Входящее сообщение от пользователя

```
Telegram Update
    │
    ▼
middleware (rate-limit: 5 запросов/мин/user)
    │
    ▼
HandlerTypeMessageText
    │
    ├── "/start" → HandleMessage (простое приветствие)
    │
    └── RegisterHandlerMatchFunc (все остальные сообщения)
         │
         ├── В сообщении есть @упоминание?
         │   │
         │   ├── да → isTopicCommand()?
         │   │   │
         │   │   ├── да → TopicCRUD.Handle()
         │   │   │   └── LLM парсинг → create/close/rename → Telegram API
         │   │   │
         │   │   ├── isSummaryCommand()?
         │   │   │   ├── да → SummaryHandler.Handle()
         │   │   │   │   └── checkPermission() → checkRateLimit() → LLM → reply
         │   │   │   └── нет → MentionHandler.Handle()
         │   │   │       └── LLM forward_intent → copyMessage → reply
         │   │   │
         │   └── нет → HandleMessage (автоклассификация)
         │       │
         │       ▼
         │   Предфильтр:
         │   - длина ≥ 15 слов ИЛИ
         │   - есть вложение (фото, документ)
         │       │
         │       ▼
         │   Дедупликация: processed_messages (message_id + chat_id)
         │       │
         │       ▼
         │   LLM.Classify (текст + опционально file_id кэш)
         │       │
         │   confidence ≥ 0.75 И topic ≠ current?
         │       │
         │       ├── да → forwarder.Duplicate() + ReplyWithLink()
         │       └── нет → markProcessed("low_confidence")
         │
         ▼
    Mark processed (action: forwarded / skipped / low_confidence)
```

### 2.2 Поток вебхука от omsu_setka

```
omsu_setka
    │
    POST /webhook/schedule
    Header: X-Webhook-Signature (HMAC-SHA256, hex-дайджест без префикса)
    Body: {"type":"change","group_id":12345,"changes":[...]}
    │
    ▼
WebhookHandler.Handle()
    │
    ├── verifyHMAC(body, signature) — constant-time сравнение
    │
    ├── deserialize → schedule.WebhookPayload
    │
    └── DiffEngine.ProcessWebhook()
        │
        ├── detectAnomalies() → ANOMALY_* типы
        │
        ├── saveSnapshot() → schedule_snapshots
        │
        ├── saveAnomaly() → schedule_anomalies
        │
        └── buildAnnouncement(anomalies) → send to Telegram
```

---

## 3. LLM Chain и failover

### 3.1 Структура провайдеров

Два провайдера: **Groq** (первичный, OpenAI-совместимый) + **Gemini** (Google AI Studio, фолбек).
4 специализированные цепочки (`chain` в конфиге):

| Цепочка | Приоритет | Назначение |
|---|---|---|
| `agent` | 10–40 | Agent loop (многошаговые запросы) |
| `simple` | 10–40 | Классификация, диагностика, простые ответы |
| `vision` | 10–30 | OCR, распознавание фото |
| `audio` | 10–30 | STT, голосовые сообщения |

Провайдеры сортируются по полю `priority` (меньше = выше приоритет).
Порядок в каждой цепочке: Groq → Gemini.

Подробная конфигурация — в `config.yaml` (не дублируется здесь во избежание расхождений).

### 3.2 Логика выбора провайдера (Client.Call)

```
1. Взять список всех провайдеров из Chain.Providers()
2. Для каждого провайдера:
   a. Если IsActive() == false → circuit breaker отключил → пропустить
   b. Составить список моделей:
      - [primary_model] + (FallbackModels если не skipFallbackModel)
   c. Для каждой модели:
      - Вызвать callProvider()
      - Если успех → вернуть ответ
      - Если ошибка → лог, следующая модель
   d. Пройтись по всем моделям не удалось:
      - provider.RecordFailure()
      - Если failures ≥ 3 → circuit breaker блокирует на 5 минут
      - Перейти к следующему провайдеру
3. Если все провайдеры вернули ошибку → "all providers failed"
```

### 3.3 Circuit Breaker

```go
type providerState struct {
    failures   int
    disabledAt time.Time
    disabled   bool
}
```

- 3 последовательные ошибки → `disabled = true` на 5 минут
- По истечении → автоматически возвращается в ротацию
- RecordSuccess() сбрасывает счётчик

### 3.4 Форматы API

| Тип провайдера | URL | Формат запроса | Формат ответа |
|---|---|---|---|
| `gemini` | `/v1beta/models/{model}:generateContent?key={key}` | `{contents: [{parts: [{text}]}]}` | `candidates[].content.parts[].text` |
| `deepseek` / `openai` | `/v1/chat/completions` | `{model, messages: [{role, content}]}` | `choices[].message.content` |

---

## 4. Система Persona

### 4.1 Хранение

- Основное: память (`sync.RWMutex`) + файл `prompts/persona.md`
- Seed: `prompts/persona.md` (загружается при старте, перечитывается через API reset)
- Доступен как обычный промпт через `GET/PUT /api/prompts/persona`

Формат persona.md:
```markdown
# name
Помощник

# system_prompt
Ты — помощник студенческой группы...
```

### 4.2 Использование

```go
func (c *Client) buildMessages(systemExtra, userPrompt string) []Message {
    system := c.persona.SystemPrompt()
    if systemExtra != "" {
        system += "\n\n" + systemExtra
    }
    return []Message{
        {Role: "system", Content: system},
        {Role: "user", Content: userPrompt},
    }
}
```

Persona system_prompt вставляется как `system` role в каждый LLM-вызов.

### 4.3 Управление

| Действие | API |
|---|---|
| Чтение | `GET /api/persona` |
| Обновление | `PUT /api/persona` |
| Сброс к seed | `POST /api/persona/reset` |

---

## 5. Системные промпты

### 5.1 Назначение

| Файл | LLM type | Назначение |
|---|---|---|
| `classify.txt` | `classify` | Классификация сообщения: топик + хэштеги + confidence |
| `schedule_announce.txt` | `schedule_announce` | Генерация человеческого объявления об изменениях |

### 5.2 Управление через API

| Действие | API |
|---|---|
| Список | `GET /api/prompts` |
| Чтение | `GET /api/prompts/:name` |
| Редактирование | `PUT /api/prompts/:name` |
| Удаление | `DELETE /api/prompts/:name` |

При редактировании файл перезаписывается на диске и PromptRegistry перезагружается.
SIGHUP также вызывает перезагрузку без перезапуска контейнера.

---

## 6. Forwarder (дублирование сообщений)

### 6.1 Формат шапки

```
📌 Из #общий_чат | @username
#сессия #дедлайны
— Помощник                 (если signature задан)
```

### 6.2 Логика ответа

В исходном топике бот отвечает:
```
↗️ Продублировал в «Сессия» → [ссылка]
```

Ссылка ведёт на скопированное сообщение в целевом топике.

---

## 7. Admin API

### 7.1 Эндпоинты

```
POST   /api/auth/token                   — JWT (admin_secret → token)
POST   /api/auth/logout                  — инвалидация JWT

GET    /api/persona                       — личность бота
PUT    /api/persona                       — обновить личность
POST   /api/persona/reset                 — сбросить к persona.md

GET    /api/topics?limit=&offset=         — список топиков
POST   /api/topics                        — создать топик
GET    /api/topics/:id                    — топик по ID
PUT    /api/topics/:id                    — обновить топик
DELETE /api/topics/:id                    — удалить топик
POST   /api/topics/:id/close             — закрыть топик
POST   /api/topics/:id/open              — открыть топик

GET    /api/permissions                   — права команд
PUT    /api/permissions/:command          — изменить права

GET    /api/prompts                       — список промптов
GET    /api/prompts/:name                 — содержимое промпта
PUT    /api/prompts/:name                 — обновить промпт
DELETE /api/prompts/:name                 — удалить промпт

GET    /api/config                        — глобальные настройки
PUT    /api/config                        — обновить настройки

GET    /api/stats/tokens                  — токены сегодня
GET    /api/stats/requests                — запросы сегодня
GET    /api/stats/forwards                — пересылки
GET    /api/stats/messages                — сообщения
GET    /api/stats/providers               — провайдеры
GET    /api/stats/check-providers         — проверка доступности
POST   /api/stats/test-model              — тест LLM модели

POST   /api/bot/send                      — отправить сообщение в группу

GET    /api/schedule/snapshots?limit=&offset=  — снэпшоты
GET    /api/schedule/anomalies?limit=&offset=   — аномалии

POST   /api/admin/superadmins             — добавить суперадмина
DELETE /api/admin/superadmins/:user_id    — удалить суперадмина
POST   /api/admin/groups/register-webhooks — регистрация вебхуков
POST   /api/admin/sync-trigger            — запустить синхронизацию setka
```

### 7.2 Формат ответов

**Успех:**
```json
{"success": true, "data": {...}}
```

**Список с пагинацией:**
```json
{"success": true, "data": [...], "meta": {"total": 42, "limit": 20, "offset": 0}}
```

**Ошибка:**
```json
{"success": false, "error": {"code": "NOT_FOUND", "message": "topic not found"}}
```

### 7.3 Коды ошибок

| HTTP | Code | Когда |
|---|---|---|
| 400 | `INVALID_REQUEST` | Невалидный JSON |
| 401 | `UNAUTHORIZED` | Нет Authorization |
| 401 | `INVALID_TOKEN` | JWT невалиден |
| 403 | `FORBIDDEN` | Недостаточно прав |
| 404 | `NOT_FOUND` | Ресурс не найден |
| 409 | `CONFLICT` | Дубликат |
| 422 | `VALIDATION_ERROR` | Поле не прошло валидацию |
| 500 | `INTERNAL_ERROR` | Ошибка сервера |

### 7.4 Аутентификация

1. `POST /api/auth/token` с `{"admin_secret": "..."}` → получаем JWT (HS256, 4h)
2. Все остальные эндпоинты требуют `Authorization: Bearer <JWT>`

---

## 8. Дедупликация

Таблица `processed_messages`:
```sql
UNIQUE(message_id, chat_id)
```

- Проверяется ДО LLM-вызова (экономия токенов)
- Действия: `forwarded`, `skipped`, `low_confidence`
- При повторном получении того же message_id — игнорируется

---

## 9. Rate Limiting

| Лимит | Где | Значение |
|---|---|---|
| Глобальный | `internal/handler/middleware.go` | 5 запросов/мин/user **(не подключён — см. `cmd/bot/main.go`)** |
| API General | `internal/api/router.go` | 120 запросов/мин/IP |
| API Search | `internal/api/router.go` | 30 запросов/мин/IP |
| Саммари | `summary_requests` таблица | 1 запрос/30 мин/user |
| LLM дневной | `Tracker.dailyTokens` | 100 000 токенов/день |

При 80% дневного лимита — алерт в лог.
При 100% — автоклассификация отключается (ручные команды работают).

---

## 10. Интеграция с omsu_setka

### 10.1 Регистрация вебхука

При старте бот регистрирует себя в omsu_setka:
```json
POST /api/v1/admin/webhooks
{
  "url": "http://groupbot:8081/webhook/schedule",
  "secret": "...",
  "group_ids": [12345],
  "enabled": true
}
```

- `group_ids` — ID группы ОмГУ, setka фильтрует изменения только для неё
- `secret` — HMAC-ключ для подписи

### 10.2 Проверка подписи

```go
func verifyHMAC(body []byte, signature string) bool {
    mac := hmac.New(sha256.New, secret)
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal(expected, signature)
}
```

Заголовок: `X-Webhook-Signature`.

### 10.3 Типы аномалий

| Тип | Условие |
|---|---|
| `ANOMALY_BUILDING` | Изменился корпус (`field: "building"`) |
| `ANOMALY_ROOM` | Изменилась аудитория (`field: "room"`) |
| `ANOMALY_SUBJECT` | Изменился предмет (`field: "subject"`) |
| `ANOMALY_CANCEL` | Пара удалена (`field: "full"`, old != "", new == "") |

---

## 11. База данных

### 11.1 Таблицы

| Таблица | Назначение |
|---|---|---|
| `topics` | Топики форума |
| `processed_messages` | Дедупликация сообщений |
| `llm_requests` | Логи LLM-вызовов |
| `schedule_snapshots` | Снэпшоты расписания |
| `schedule_anomalies` | Обнаруженные аномалии |
| `command_permissions` | Права команд |
| `summary_requests` | Rate-limit саммари |

*Примечание: `bot_persona` не существует как SQLite-таблица — персона хранится в памяти + файле.*

---

## 12. Промпт-инжиниринг

### 12.1 Формат промптов

Промпты — это текстовые файлы с плейсхолдерами:
```
{topics}  — список топиков (name + description)
{text}    — текст сообщения пользователя
{changes} — JSON с изменениями расписания
```

### 12.2 Ожидаемый ответ LLM

Все промпты требуют JSON-ответ строгой структуры:

**classify.txt:**
```json
{"topic": "session", "hashtags": ["экзамен"], "confidence": 0.92}
```

**forward_intent.txt:**
```json
{"intent": "forward", "target_topic": "session", "confidence": 0.9}
```

**topic_command.txt:**
```json
{"intent": "create", "topic_name": "Практика", "new_name": "", "confidence": 0.95}
```

---

## 13. Ключевые паттерны

### 13.1 Graceful degradation

| Сценарий | Поведение |
|---|---|
| Telegram API недоступен | HTTP-сервер продолжает работать, LLM-тесты доступны |
| LLM провайдер упал | Failover на следующий провайдер |
| Все провайдеры недоступны | Возврат ошибки, логирование |
| SQLite недоступен | Процесс падает (восстанавливается Docker) |

### 13.2 Nil-безопасность

Все ключевые зависимости могут быть nil при старте:
- `tgBot == nil` → бот не стартует, API работает
- `poster == nil` → schedule/webhook не отправляют в Telegram
- `Chain.Pick()` падает без провайдеров → возвращает ошибку

Проверка через `BotSender` interface (nil-interface != interface с nil pointer).

### 13.3 Конфигурация

Иерархическая YAML + env-переменные (cleanenv):
- `config.yaml` — основные настройки
- `.env` — переопределение (подстановка `${VAR}` в YAML через `os.ExpandEnv`)
- `CONFIG_PATH` — кастомный путь до config.yaml

### 13.4 Graceful Shutdown

```go
ctx, cancel := signal.NotifyContext(context.Background(), SIGINT, SIGTERM)
tgBot.Start(ctx)  // блокируется до сигнала
apiServer.App.Shutdown()
```

SIGHUP — перезагрузка промптов без рестарта.

---

## 14. Структура проекта

```
omsu_bot/
├── cmd/bot/main.go              ← точка входа
├── internal/
│   ├── api/                     ← REST API (Fiber + JWT)
│   │   ├── router.go            ← маршруты + Server struct
│   │   ├── middleware_auth.go   ← JWT аутентификация
│   │   ├── response.go          ← единый формат ответов
│   │   ├── handler_persona.go
│   │   ├── handler_topics.go
│   │   ├── handler_permissions.go
│   │   ├── handler_prompts.go
│   │   ├── handler_stats.go
│   │   ├── handler_schedule.go
│   │   ├── handler_diagnostic.go
│   │   ├── handler_config.go
│   │   ├── handler_superadmin.go
│   │   └── context_handler.go
│   ├── classifier/              ← LLM классификация
│   ├── config/                  ← cleanenv конфиг
│   ├── db/                      ← SQLite + миграции
│   ├── forwarder/               ← дублирование сообщений
│   ├── handler/                 ← Telegram handlers
│   │   ├── handler_message.go   ← автоклассификация
│   │   ├── handler_mention.go   ← @bot команды + agent loop
│   │   ├── handler_webhook.go   ← вебхук от setka
│   │   ├── handler_settings.go  ← настройки группы
│   │   ├── antispam.go          ← капча, flood control, фильтр ссылок
│   │   └── middleware.go        ← rate-limit (не подключён)
│   ├── llm/                     ← LLM chain + circuit breaker
│   │   ├── client.go            ← HTTP клиент + failover
│   │   ├── provider.go          ← Provider + Chain
│   │   ├── tracker.go           ← дневной лимит токенов
│   │   └── prompts.go           ← загрузка промптов
│   ├── persona/                 ← Store + seed_parser
│   └── schedule/                ← diff engine + announcer
├── admin/                       ← React SPA (admin panel)
├── prompts/                     ← *.txt/*.md файлы промптов (включая persona.md)
├── config.yaml
├── Dockerfile
└── docker-compose*.yml
```
