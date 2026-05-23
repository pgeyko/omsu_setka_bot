# omsu_bot — Technical Specification

> Техническая справка для агентов. За функциональными требованиями —
> в `../GROUPBOT_TZ.md`. Этот файл описывает контракты, схемы и детали реализации.

---

## SQLite Schema (полная)

```sql
-- Личность бота (singleton, id всегда = 1)
CREATE TABLE bot_persona (
    id            INTEGER PRIMARY KEY CHECK (id = 1),
    name          TEXT NOT NULL DEFAULT 'Помощник',
    system_prompt TEXT NOT NULL DEFAULT '',
    signature     TEXT NOT NULL DEFAULT '',
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

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
```

---

## Webhook Contract (omsu_mirror → omsu_bot)

**Endpoint:** `POST /webhook/schedule`

**Headers:**
```
Content-Type: application/json
X-Webhook-Signature: sha256=<hex(HMAC-SHA256(body, SCHEDULE_WEBHOOK_SECRET))>
```

**Payload:**
```json
{
  "type": "change",
  "group_id": 12345,
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

**Response:** `200 OK` (всегда, даже если `group_id` не совпадает с конфигом).

---

## LLM Provider Chains (4 специализированных цепочки)

Два провайдера: **Groq** (первичный, OpenAI-совместимый) + **Gemini** (Google AI Studio, фолбек).
Конфигурация: `config.yaml` → поле `chain` группирует провайдеры по цепочкам.

### Agent chain (agent_loop — сложные многошаговые)
```
llama-3.3-70b-versatile  (Groq, 30 RPM, 1K RPD, 12K TPM)
  → gemma-4-31b-it        (Gemini, 15 RPM, 1.5K RPD)
    → qwen/qwen3-32b      (Groq, 60 RPM, 1K RPD, 6K TPM)
      → gemma-4-26b-a4b-it (Gemini, 15 RPM, 1.5K RPD)
```

### Simple chain (classify, diagnostic, summary)
```
qwen/qwen3-32b            (Groq, 60 RPM, 1K RPD, 6K TPM)
  → gemini-3.1-flash-lite  (Gemini, 15 RPM, 500 RPD, 250K TPM)
    → llama-3.1-8b-instant (Groq, 30 RPM, 14.4K RPD)
      → gemma-4-26b-a4b-it (Gemini, 15 RPM, 1.5K RPD)
```

### Vision chain (OCR, фото)
```
llama-4-scout-17b-16e      (Groq, 30 RPM, 1K RPD, 30K TPM)
  → gemini-3.1-flash-lite  (Gemini, 15 RPM, 500 RPD, 250K TPM)
    → gemma-4-31b-it        (Gemini, 15 RPM, 1.5K RPD)
```

### Audio chain (STT, голосовые)
```
gemini-3.1-flash-lite     (Gemini, 15 RPM, 500 RPD, 250K TPM)
  → whisper-large-v3-turbo (Groq, 20 RPM, 2K RPD)
    → whisper-large-v3     (Groq, 20 RPM, 2K RPD)
```

**Маршрутизация:** `PickChain(taskType, requiresVision)` → `agent_loop`→agent, `ocr`→vision, `stt`→audio, default→simple.

**Retry:** agent_loop ретраится до 2 раз с задержкой 3s/6s при исчерпании всех провайдеров.

**Rate limiter:** RPM/TPM/RPD/TPD скользящее окно. Circuit breaker: 3 ошибки подряд → disabled на 5 мин.

---

## Persona System

**persona.md формат:**
```markdown
# name
Вероника

# system_prompt
Ты — Вероника (Ника), технический ассистент студенческой группы.
...
```

**Загрузка:**
```
1. SELECT * FROM bot_persona WHERE id = 1
2. Если нет записи → прочитать persona.md → INSERT
3. Кэшировать в persona.Store
```

---

## Message Processing Flow

```
Telegram Update
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

Ограниченные инструменты проверяют права через `AdminChecker.IsAdmin()`.

---

## Admin REST API (Fiber, порт 8081)

### Auth
```
POST /api/auth/token     Body: {"secret": "..."}     → {"token": "jwt..."}
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
POST /api/stats/test-model  Body: {"provider": "...", "prompt": "..."}
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
