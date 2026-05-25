# omsu_bot — Technical Specification

> Техническая справка для агентов. За функциональными требованиями —
> в `../GROUPBOT_TZ.md`. Этот файл описывает контракты, схемы и детали реализации.

---

## SQLite Schema (полная)

```sql
-- Группы (мультиарендность)
CREATE TABLE groups (
    chat_id       INTEGER PRIMARY KEY,
    title         TEXT NOT NULL,
    api_token     TEXT NOT NULL UNIQUE,
    omsu_group_id INTEGER NOT NULL DEFAULT 0,
    is_active     INTEGER NOT NULL DEFAULT 1,
    is_vip        INTEGER NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Суперадмины (имеют доступ ко всем группам)
CREATE TABLE superadmins (
    user_id    INTEGER PRIMARY KEY,
    note       TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Топики форум-группы
CREATE TABLE topics (
    id           INTEGER PRIMARY KEY,
    group_id     INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
    tg_thread_id INTEGER NOT NULL,
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL,
    aliases      TEXT NOT NULL DEFAULT '[]',    -- JSON []string
    description  TEXT NOT NULL DEFAULT '',
    hashtags     TEXT NOT NULL DEFAULT '[]',    -- JSON []string
    is_active    INTEGER NOT NULL DEFAULT 1,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(group_id, tg_thread_id),
    UNIQUE(group_id, slug)
);

-- Дедупликация обработанных сообщений
CREATE TABLE processed_messages (
    id               INTEGER PRIMARY KEY,
    message_id       INTEGER NOT NULL,
    chat_id          INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
    thread_id        INTEGER,
    action           TEXT NOT NULL,             -- "forwarded" | "skipped" | "low_confidence" | "duplicate"
    target_thread_id INTEGER,
    processed_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(message_id, chat_id)
);

-- Логи LLM-запросов
CREATE TABLE llm_requests (
    id            INTEGER PRIMARY KEY,
    group_id      INTEGER REFERENCES groups(chat_id) ON DELETE SET NULL,
    type          TEXT NOT NULL,                -- "classify" | "agent_loop" | "ocr" | "stt" | "summary" | "schedule_query"
    provider      TEXT NOT NULL DEFAULT '',
    input_tokens  INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    model         TEXT NOT NULL,
    cost_usd      REAL NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Буфер сообщений (для саммари + поиска по тегам)
CREATE TABLE message_buffer (
    chat_id    INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
    thread_id  INTEGER NOT NULL,
    message_id INTEGER NOT NULL DEFAULT 0,
    username   TEXT NOT NULL,
    text       TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Хэштеги сообщений (LLM-сгенерированные при пересылке)
CREATE TABLE message_tags (
    id         INTEGER PRIMARY KEY,
    chat_id    INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
    message_id INTEGER NOT NULL,
    tag        TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Снапшоты расписания
CREATE TABLE schedule_snapshots (
    id         INTEGER PRIMARY KEY,
    group_id   INTEGER NOT NULL DEFAULT 0,
    data       TEXT NOT NULL,                   -- JSON
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Аномалии расписания
CREATE TABLE schedule_anomalies (
    id          INTEGER PRIMARY KEY,
    snapshot_id INTEGER NOT NULL REFERENCES schedule_snapshots(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,                  -- "ANOMALY_BUILDING" | "ANOMALY_ROOM" | "ANOMALY_SUBJECT" | "ANOMALY_CANCEL"
    details     TEXT NOT NULL,                  -- JSON
    notified    INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Права команд (пер-группа)
CREATE TABLE command_permissions (
    group_id     INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
    command      TEXT NOT NULL,
    allowed_role TEXT NOT NULL DEFAULT 'everyone',  -- "everyone" | "admin"
    PRIMARY KEY (group_id, command)
);

-- Rate limit для саммари
CREATE TABLE summary_requests (
    user_id      INTEGER NOT NULL,
    chat_id      INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
    requested_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, chat_id)
);

-- Конфигурация времени выполнения (key-value)
CREATE TABLE bot_config (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT ''
);

-- JWT-черный список
CREATE TABLE revoked_tokens (
	jti        TEXT PRIMARY KEY,
	expires_at INTEGER NOT NULL
);

-- Webhook replay protection (P1#11)
CREATE TABLE webhook_events (
	event_id    TEXT PRIMARY KEY,
	received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at  DATETIME NOT NULL
);

-- Media group items for album forwarding (авто-объединение альбомов)
CREATE TABLE media_group_items (
	media_group_id TEXT NOT NULL,
	message_id     INTEGER NOT NULL,
	chat_id        INTEGER NOT NULL,
	file_id        TEXT NOT NULL DEFAULT '',
	caption        TEXT NOT NULL DEFAULT '',
	created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (media_group_id, message_id)
);
```

---

## Webhook Contract (omsu_setka → omsu_bot)

[omsu_setka](https://github.com/pgeyko/omsu_setka) — отдельный проект. Интеграция через HTTP webhook.

**Endpoint:** `POST /webhook/schedule`

**Headers:**
```
Content-Type: application/json
X-Webhook-Signature: <hex(HMAC-SHA256(timestamp + "." + body, SCHEDULE_WEBHOOK_SECRET))>
X-Webhook-Timestamp: <RFC3339>
X-Webhook-Event-ID: <hex(16 random bytes)>
```

**Payload:**
```json
{
  "type": "change",
  "group_id": 12345,
  "entity_type": "group",
  "entity_id": 12345,
  "event_id": "a1b2c3d4e5f6...",
  "occurred_at": "2026-05-24T12:00:00Z",
  "changes": [
    {
      "date": "2025-03-18",
      "pair": 3,
      "field": "building",
      "old": "4",
      "new": "1",
      "subject": "История"
    }
  ]
}
```

**Validation:**
- HMAC-SHA256 проверяется по схеме `timestamp + "." + body` (если `X-Webhook-Timestamp` присутствует)
- Clock skew: не более 5 минут
- Dedup по `event_id`: повторные события отвергаются (TTL 24ч)
- `entity_type != "group"` → `200 skipped` (пока поддерживаются только группы)

**Response:** `200 OK` / `200 {"status":"duplicate"}` / `401 Unauthorized`

---

## LLM Provider Chains (4 специализированных цепочки)

Три провайдера: **Groq** (первичный, OpenAI-совместимый) → **Gemini** (Google AI Studio, фолбек) → **OpenRouter** (free tier, last resort).
Конфигурация: `config.yaml` → поле `chain` группирует провайдеры по цепочкам. Каждый провайдер имеет key2 дубль (второй Groq API key) для round-robin при исчерпании TPM/TPD.

### Agent chain (agent_loop — многошаговые запросы)
```
llama-3.3-70b-versatile     (Groq x2 key, 120 RPM, 2K RPD, 12K TPM)
  → qwen/qwen3-32b          (Groq x2 key, 120 RPM, 2K RPD, 12K TPM)
    → gemma-4-31b-it         (Gemini, 30 RPM, 3K RPD)
      → gemma-4-26b-a4b-it   (Gemini, 30 RPM, 3K RPD)
        → openai/gpt-oss-120b:free (OpenRouter, catch-all)
          → openrouter/free         (OpenRouter, catch-all)
```

### Simple chain (classify, diagnostic, summary)
```
llama-3.1-8b-instant       (Groq x2 key, 120 RPM, 2K RPD, 12K TPM)
  → qwen/qwen3-32b         (Groq x2 key, 60 RPM, 2K RPD, 12K TPM)
    → gemini-3.1-flash-lite (Gemini, 30 RPM, 3K RPD, 500K TPM)
      → gemini-3.1-flash-lite (Gemini, 30 RPM, 3K RPD, 500K TPM, fallback)
        → openai/gpt-oss-20b:free (OpenRouter, catch-all)
          → openrouter/free         (OpenRouter, catch-all)
```

### Vision chain (OCR, фото)
```
llama-4-scout-17b-16e      (Groq x2 key, 120 RPM, 2K RPD, 30K TPM)
  → gemini-3.1-flash-lite  (Gemini, 30 RPM, 3K RPD, 500K TPM)
    → gemma-4-31b-it        (Gemini, 30 RPM, 3K RPD)
      → gemma-4-26b-a4b-it  (Gemini, 30 RPM, 3K RPD)
```

### Audio chain (STT, голосовые)
```
gemini-3.1-flash-lite     (Gemini, 30 RPM, 3K RPD, 500K TPM)
  → whisper-large-v3-turbo (Groq x2 key, 40 RPM, 4K RPD)
    → whisper-large-v3     (Groq x2 key, 40 RPM, 4K RPD)
```

**Маршрутизация:** `PickChain(taskType, requiresVision)` → `agent_loop`→agent, `ocr`→vision, `stt`→audio, default→simple.

**Retry:** agent_loop ретраится до 2 раз с задержкой 3s/6s при исчерпании всех провайдеров.

**Rate limiter:** RPM/TPM/RPD/TPD скользящее окно. Circuit breaker: 3 ошибки подряд → disabled на 5 мин.

---

## Persona Storage

Бот хранит личность в памяти + файле `prompts/persona.md`, НЕ в SQLite.
1. При старте: файл prompts/persona.md → парсинг → кэш в memory (sync.RWMutex)
3. POST /api/persona/reset → перечитывание prompts/persona.md
```

---

## Message Processing Flow

```
Telegram Update
     │
     ▼
  Rate-limit middleware (cfg.RateLimit.GlobalPerUserPerMin, P1#6)
     │
     ▼
  Antispam (captcha/flood/links check)
     │
     ▼
  Session check (admin settings flow?)
  ├─ да → HandleAdminInput
  └─ нет
        │
        ▼
      Voice/Photo processing (если включено)
      ├─ voice → audio chain (gemini-3.1-flash-lite STT)
      ├─ photo → vision chain (gemma-4-31b-it → 3.1-flash-lite OCR)
      └─ text → continue
            │
            ▼
          Переключение маршрута:
          ├─ Slash command (/help, /status, /tag, /topics, /resend, /register, /settings)
          │   → handleSlashCommand
          ├─ Mention (@botusername) или алиас (имя/народ/ребята)
          │   → MentionHandler → AgentOrchestrator (LLM + tools)
          └─ Обычное сообщение
                → Handler.HandleMessage (classify → forward)
```

---

## Agent Tools (через AgentOrchestrator)

| Tool | Описание | Доступ |
|---|---|---|
| `get_schedule` | Расписание на день | Всем |
| `generate_summary` | Саммари топика | Всем |
| `manage_topic` | Создать/закрыть/переименовать | Админам |
| `moderate_user` | Мут/бан/размут | Админам (владелец immune) |
| `run_protocol` | Протоколы (зачистка) | Админам |

### Enforcement (P1#7)

Права проверяются в два слоя:
1. **Hardcoded** `restrictedTools` map в `orchestrator.go` — legacy fallback
2. **DB-backed** `permissions.Service` — читает `command_permissions` таблицу

`canExecuteTool(ctx, chatID, userID, toolName)` → проверяет оба слоя.
Фильтрация происходит на этапе загрузки инструментов для LLM (pre-filter) и при выполнении (runtime).
Администратор/владелец группы всегда bypass.

### PermissionService (`internal/permissions/service.go`)
```go
type Service struct { db *sql.DB }
func (s *Service) IsAdminOnly(ctx, groupID, command) bool
func (s *Service) AllowedRole(ctx, groupID, command) string
```

---

## Admin REST API (Fiber, порт 8081)

Конфигурация:
- Body limit: 512 KB (глобально, соответствует контекстным роутам)
- CORS: ограничен `CORS_ORIGIN` (в production не `*`)
- Rate limit: 120 req/min/IP (api_general), 30 req/min/IP (api_search)
- Таймауты: Read 10s, Write 10s

### Auth
```
POST /api/auth/token     Body: {"admin_secret": "..."}     → {"token": "jwt..."}
POST /api/auth/logout
```

### Groups
```
GET    /api/groups
POST   /api/groups          Body: {chat_id, title, api_token, omsu_group_id, is_active, is_vip}
GET    /api/groups/:chat_id
PUT    /api/groups/:chat_id
DELETE /api/groups/:chat_id
```

### Group Context
```
GET/PUT /api/groups/:chat_id/context/persona
GET/PUT /api/groups/:chat_id/context/system-prompt
GET/PUT /api/groups/:chat_id/context/knowledge
GET/PUT /api/groups/:chat_id/context/features
```

### Topics
```
GET    /api/topics?group_id=
POST   /api/topics
GET    /api/topics/:id
PUT    /api/topics/:id
DELETE /api/topics/:id
POST   /api/topics/:id/close
POST   /api/topics/:id/open
```

### Permissions
```
GET /api/permissions?group_id=
PUT /api/permissions/:command?group_id=  Body: {"allowed_role": "everyone"|"admin"}
```

### Prompts
```
GET /api/prompts
GET /api/prompts/:name
PUT /api/prompts/:name
DELETE /api/prompts/:name
```

### Config
```
GET /api/config
PUT /api/config     Body: {skip_fallback_model, global_voice_transcription, global_photo_processing}
```

### Stats
```
GET /api/stats/tokens
GET /api/stats/requests
GET /api/stats/forwards
GET /api/stats/messages
GET /api/stats/providers
POST /api/stats/test-model         Body: {"provider": "...", "prompt": "..."}
POST /api/stats/test-all-models    — тест всех LLM провайдеров (группировка по типу)
```

### Schedule
```
GET /api/schedule/snapshots
GET /api/schedule/anomalies
```

### Bot
```
POST /api/bot/send  Body: {"chat_id": ..., "text": "...", "thread_id": ...}
```

### Superadmin
```
GET    /api/admin/superadmins
POST   /api/admin/superadmins        Body: {"user_id": ..., "note": "..."}
DELETE /api/admin/superadmins/:user_id
POST   /api/admin/groups/register-webhooks
```

### Frontend
```
GET /admin → React SPA (admin/dist/)
GET /swagger/* (только dev)
```
