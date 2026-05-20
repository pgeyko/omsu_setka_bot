# Пятница (GroupBot)

Telegram-бот для студенческой группы. Личность — «Пятница», ИИ-ассистент в роли старшей сестры. Автоматическая классификация и пересылка сообщений, расписание через omsu_setka, настраиваемая личность, REST API + Admin UI.

---

## Архитектура

```
Telegram (супергруппа с топиками)
    │
    ├── long polling ──▶ GroupBot (:8081)
    │                       ├── REST API (Fiber + JWT)
    │                       ├── Admin UI (React SPA)
    │                       ├── Swagger UI
    │                       └── SQLite
    │
    └── omsu_setka (:8080) ──POST /webhook/schedule (HMAC)──▶ GroupBot
                                  │
                              GET /schedule/group/{id}/day
                                  │
                              ◀── GroupBot (запрос расписания)
```

---

## Быстрый старт

### 1. Получить токен бота

Создай бота через [@BotFather](https://t.me/BotFather), получи `BOT_TOKEN`.

### 2. Создать супергруппу с топиками

1. Создай forum-супергруппу, включи «Темы».
2. Добавь бота → сделай администратором.
3. Определи `GROUP_ID` (с минусом: `-100...`).
4. Запиши `OMSU_GROUP_ID` (ID группы в ОмГУ).

### 3. Настроить окружение

```bash
cp .env.example .env
# Заполни: BOT_TOKEN, GROUP_ID, OMSU_GROUP_ID, GEMINI_API_KEY, ADMIN_SECRET, JWT_SECRET
```

### 4. Запустить

```bash
# Production
docker compose up --build -d

# Development (оба сервиса: бот + omsu_mirror)
docker compose -f docker-compose.local.yml up --build -d
```

---

## Команды боту

### Через @username

| Пример | Что делает |
|---|---|
| `@bot перешли в сессию` | Переслать сообщение в топик (reply или само сообщение) |
| `@bot создай топик Практика` | Создать новый топик (админ) |
| `@bot зарегистрируй этот топик как Тест` | Зарегистрировать существующий топик (админ) |
| `@bot закрой топик сессия` | Закрыть топик (админ) |
| `@bot переименуй топик сессия в Экзамены` | Переименовать (админ) |
| `@bot саммари` / `@bot что пропустил?` | Саммари сообщений в топике |
| `@bot расписание на завтра?` | Расписание через omsu_setka |

### Через слэш-команды

| Команда | Что делает |
|---|---|
| `/start` | Приветствие |
| `/help` | Справка |
| `/id` | ID текущего топика |
| `/topics` | Список топиков |
| `/resend [slug]` | Переслать ответное сообщение в топик |
| `/register [название]` | Зарегистрировать этот топик (админ) |
| `/summary` | Саммари сообщений |
| `/status` | Состояние бота |

### По имени

Любое сообщение, содержащее «Пятница» или алиасы, обрабатывается ботом:

| Сообщение | Реакция |
|---|---|
| «Пятница, привет!» | Приветствие в характере |
| «Пятница, скинь расписание» | **Расписание через omsu_setka** |
| «Пятница, зарегистрируй топик как тест» | Регистрация топика (админ) |
| «Пятница, перешли в тест» | Пересылка сообщения |
| «Пятница, саммари» | Саммари |

---

## Интеграция с omsu_setka

### Вебхук изменений расписания

При старте бот регистрирует себя в omsu_setka:

```
POST /api/v1/admin/webhooks
Body: {"url":"http://groupbot:8081/webhook/schedule","secret":"...","group_ids":[12345],"enabled":true}
```

- HMAC-SHA256 подпись в заголовке `X-Webhook-Signature`
- Фильтр по `group_ids` — вебхуки только для вашей группы

### Запрос расписания

При запросе «расписание на завтра» бот:
1. Парсит дату через LLM
2. Делает `GET /api/v1/schedule/group/{id}/day?date=YYYY-MM-DD` к omsu_mirror
3. Подаёт реальные данные в LLM для ответа

---

## API Reference

### Формат ответов

```json
// Успех
{"success": true, "data": {...}}

// Список с пагинацией
{"success": true, "data": [...], "meta": {"total": 42, "limit": 20, "offset": 0}}

// Ошибка
{"success": false, "error": {"code": "NOT_FOUND", "message": "topic not found"}}
```

### Auth

| Метод | Эндпоинт | Описание |
|---|---|---|
| POST | `/api/auth/token` | Получить JWT (body: `admin_secret`) |

### Persona

| Метод | Эндпоинт |
|---|---|
| GET | `/api/persona` |
| PUT | `/api/persona` |
| POST | `/api/persona/reset` |

### Topics

| Метод | Эндпоинт |
|---|---|
| GET | `/api/topics?limit=&offset=` |
| POST | `/api/topics` |
| GET | `/api/topics/:id` |
| PUT | `/api/topics/:id` |
| DELETE | `/api/topics/:id` |
| POST | `/api/topics/:id/close` |
| POST | `/api/topics/:id/open` |

### Permissions

| Метод | Эндпоинт |
|---|---|
| GET | `/api/permissions` |
| PUT | `/api/permissions/:command` |

### Prompts

| Метод | Эндпоинт |
|---|---|
| GET | `/api/prompts` |
| GET | `/api/prompts/:name` |
| PUT | `/api/prompts/:name` |
| DELETE | `/api/prompts/:name` |

### Config

| Метод | Эндпоинт |
|---|---|
| GET | `/api/config` |
| PUT | `/api/config` |

### Stats / Diagnostics

| Метод | Эндпоинт |
|---|---|
| GET | `/api/stats/tokens` |
| GET | `/api/stats/requests` |
| GET | `/api/stats/forwards` |
| GET | `/api/stats/messages` |
| GET | `/api/stats/providers` |
| GET | `/api/stats/check-providers` |
| POST | `/api/stats/test-model` |

### Bot

| Метод | Эндпоинт |
|---|---|
| POST | `/api/bot/send` |

### Schedule

| Метод | Эндпоинт |
|---|---|
| GET | `/api/schedule/snapshots?limit=&offset=` |
| GET | `/api/schedule/anomalies?limit=&offset=` |

### Коды ошибок

| HTTP | Code |
|---|---|
| 400 | `INVALID_REQUEST` |
| 401 | `UNAUTHORIZED` / `INVALID_TOKEN` |
| 403 | `FORBIDDEN` |
| 404 | `NOT_FOUND` |
| 409 | `CONFLICT` |
| 422 | `VALIDATION_ERROR` |
| 429 | `RATE_LIMITED` |
| 500 | `INTERNAL_ERROR` |

---

## Admin UI

В браузере: `http://localhost:8081/admin/`

Разделы:
| Страница | Путь | Описание |
|---|---|---|
| Личность | `/` | Имя, подпись, системный промпт |
| Топики | `/topics` | CRUD + пагинация |
| Действия | `/permissions` | Права команд (все / админ) |
| Промпты | `/prompts` | Редактирование .txt файлов |
| Статистика | `/stats` | Токены, запросы, провайдеры |
| Диагностика | `/diagnostics` | Проверка провайдеров, тест модели, отправка в группу |
| Расписание | `/schedule` | Снэпшоты и аномалии |

---

## Переменные окружения

| Переменная | По умолчанию | Описание |
|---|---|---|
| `BOT_TOKEN` | — | Токен Telegram бота |
| `GROUP_ID` | — | ID группы (с минусом) |
| `OMSU_GROUP_ID` | 0 | ID группы в ОмГУ |
| `GEMINI_API_KEY` | — | Gemini |
| `DEEPSEEK_API_KEY` | — | DeepSeek |
| `GEMINI_RESERVE_API_KEY` | — | Резервный Gemini |
| `ADMIN_SECRET` | — | Для JWT |
| `JWT_SECRET` | — | Подпись JWT |
| `SCHEDULE_WEBHOOK_SECRET` | — | HMAC-ключ вебхука |
| `DB_PATH` | `./data/groupbot.db` | SQLite |
| `LOG_LEVEL` | `info` | debug / info / warn / error |
| `LOG_FORMAT` | `text` | text / json |
| `APP_ENV` | `development` | production закрывает Swagger |
| `SWAGGER_ENABLED` | `false` | Swagger UI |
| `SETKA_BASE_URL` | — | URL omsu_mirror (API) |
| `SETKA_ADMIN_KEY` | — | Admin-ключ omsu_mirror |
| `SETKA_PUBLIC_URL` | — | Публичный URL setka (для ссылок) |
| `LLM_SKIP_FALLBACK_MODEL` | `false` | Пропускать fallback-модели |

---

## Безопасность

| Защита | Статус |
|---|---|
| JWT Auth (HS256, 24h) | ✅ |
| Rate limit API (120 req/min) | ✅ |
| Rate limit Telegram (5 req/min/user) | ✅ |
| Security Headers (CSP, HSTS) | ✅ |
| Body Limit (1 KB) | ✅ |
| Read/Write Timeouts (5s) | ✅ |
| HMAC webhook (SHA-256) | ✅ |
| SQL-параметризация | ✅ |
| Non-root Docker | ✅ |
| Trusted Proxies | ✅ |

---

## Разработка

```bash
# Локально
go run ./cmd/bot/main.go

# Docker dev (оба сервиса)
docker compose -f docker-compose.local.yml up --build -d

# Логи
docker logs -f groupbot-dev

# Тесты
go test ./... -count=1 -v

# Линтер
go vet ./...

# Сборка
go build -ldflags="-s -w" -trimpath -o groupbot ./cmd/bot/main.go
```

### Логирование

`LOG_LEVEL=debug` включает:
- Входящие Telegram-сообщения (msg_id, username, text)
- LLM-вызовы (провайдер, модель, полный промпт, полный ответ)
- HTTP-запросы к API (метод, path, статус, latency)
- Тело входящих вебхуков

---

## Структура проекта

```
omsu_bot/
├── cmd/bot/main.go              ← точка входа
├── internal/
│   ├── api/                     ← REST API (Fiber, JWT, rate limit)
│   ├── classifier/              ← LLM-классификация
│   ├── config/                  ← cleanenv (YAML + env)
│   ├── db/                      ← SQLite + миграции
│   ├── forwarder/               ← дублирование сообщений
│   ├── handler/                 ← Telegram handlers + саммари
│   ├── llm/                     ← LLM chain + failover + circuit breaker
│   ├── persona/                 ← Store + seed parser
│   └── schedule/                ← diff engine + announcer
├── admin/                       ← React SPA
├── prompts/                     ← *.txt промпты
├── commands.json                ← словари команд и алиасов
├── persona.md                   ← личность бота
├── Dockerfile
└── docker-compose*.yml
```
