# omsu_bot — GroupBot

Telegram-бот для студенческой группы с ИИ-агентом. Работает в форум-супергруппе (топики).

**Возможности:**
- Классификация сообщений и авто-пересылка в правильные топики
- ИИ-агент с tool calling (расписание, топики, модерация, саммари)
- Расписание занятий через webhook от [omsu_setka](https://github.com/pgeyko/omsu_setka)
- Голосовые сообщения → текст (STT через Gemini)
- Фото → текст (OCR через Gemini Vision)
- Антиспам: капча, фильтр ссылок, флуд-контроль
- Админ-панель (React SPA) + REST API

**Персона:** Вероника (Ника) — старшекурсница-куратор.

## Быстрый старт

### Требования

- Go 1.25+
- Node.js 24 (для сборки админки)
- Токены: Groq API (основной), Gemini API (фолбек), OpenRouter API (last-resort free tier), Telegram Bot

### Переменные окружения

Скопируй `.env.example` → `.env` и заполни:

```bash
# Основные
BOT_TOKEN=           # токен Telegram бота
GROUP_ID=            # ID супергруппы (отрицательное число)
GEMINI_API_KEY=      # ключ Gemini API
ADMIN_SECRET=        # пароль для админки
JWT_SECRET=          # секрет для JWT
SCHEDULE_WEBHOOK_SECRET=  # секрет для вебхуков Setka
```

### Локальный запуск

```bash
# Backend
go run ./cmd/bot/main.go

# Frontend (админка)
cd admin && npm install && npm run build
```

### Docker

```bash
docker build -t groupbot .
docker run -d --env-file .env groupbot
```

### Docker Compose

```bash
docker compose -f docker-compose.local.yml up -d
```

Запускает groupbot + omsu_setka. Для интеграции с расписанием требуется отдельно запущенный [omsu_setka](https://github.com/pgeyko/omsu_setka).

## Команды бота

| Команда | Описание |
|---|---|
| `/start` | Приветствие |
| `/help` | Все команды |
| `/id` | ID топика |
| `/topics` | Список топиков |
| `/init [id]` | Инициализация группы |
| `/tag #тег` | Поиск по хэштегу |
| `/resend [topic]` | Переслать в топик |
| `/register [name]` | Зарегистрировать топик |
| `/summary` | Саммари топика |
| `/settings` | Настройки (админ) |
| `/status` | Состояние бота |

Бот также отвечает на упоминания `@username`, обращения по имени персоны и алиасам.

## Агент (ИИ-инструменты)

| Инструмент | Доступ | Описание |
|---|---|---|
| `get_schedule` | Все | Расписание на день |
| `generate_summary` | Все | Саммари топика |
| `manage_topic` | Админ | Создание/закрытие/переименование топиков |
| `moderate_user` | Админ | Мут/бан/размут |
| `run_protocol` | Админ | Протоколы (зачистка) |

## Архитектура

```
omsu_bot/
├── cmd/bot/main.go           # Точка входа
├── internal/
│   ├── agent/                # ИИ-агент + инструменты
│   ├── api/                  # REST API (Fiber)
│   ├── buffer/               # Кольцевой буфер сообщений
│   ├── classifier/           # LLM-классификация
│   ├── config/               # Конфигурация (cleanenv)
│   ├── db/                   # SQLite + миграции
│   ├── forwarder/            # Пересылка сообщений
│   ├── handler/              # Обработчики Telegram
│   ├── llm/                  # LLM-клиент + провайдеры
│   ├── media/                # Голос/фото обработка
│   ├── messages/             # Тексты ответов (YAML)
│   ├── persona/              # Персона бота
│   ├── schedule/             # Webhook расписания
│   ├── telegram/             # Админ-кэш, синхронизация
│   └── util/                 # Утилиты (slug, даты)
├── prompts/                  # LLM-промпты (включая persona.md)
├── admin/                    # React SPA админка
├── config.yaml               # Конфигурация
├── messages.yaml             # Тексты ответов
├── protocols.json            # Протоколы агента
└── Dockerfile
```

## LLM Provider Chain

Три провайдера: **Groq** (первичный, OpenAI-совместимый) → **Gemini** (Google AI Studio, фолбек) → **OpenRouter** (free tier, last resort).
Конфигурация: `config.yaml` → поле `chain` группирует провайдеры по цепочкам.

### Agent chain (agent_loop — многошаговые запросы)
```
llama-3.3-70b-versatile  (Groq, 120 RPM, 2K RPD, 12K TPM)
  → qwen/qwen3-32b       (Groq, 120 RPM, 2K RPD, 12K TPM)
    → gemma-4-31b-it      (Gemini, 30 RPM, 3K RPD)
      → gemma-4-26b-a4b-it(Gemini, 30 RPM, 3K RPD)
        → openai/gpt-oss-120b:free (OpenRouter, catch-all)
          → openrouter/free        (OpenRouter, catch-all)
```

### Simple chain (classify, diagnostic, summary)
```
llama-3.1-8b-instant      (Groq, 120 RPM, 2K RPD, 12K TPM)
  → qwen/qwen3-32b        (Groq, 60 RPM, 2K RPD, 12K TPM)
    → gemini-3.1-flash-lite (Gemini, 30 RPM, 3K RPD, 500K TPM)
      → gemini-3.1-flash-lite (Gemini, 30 RPM, 3K RPD, 500K TPM, fallback)
        → openai/gpt-oss-20b:free (OpenRouter, catch-all)
          → openrouter/free        (OpenRouter, catch-all)
```

### Vision chain (OCR, фото)
```
llama-4-scout-17b-16e      (Groq, 120 RPM, 2K RPD, 30K TPM)
  → gemini-3.1-flash-lite  (Gemini, 30 RPM, 3K RPD, 500K TPM)
    → gemma-4-31b-it        (Gemini, 30 RPM, 3K RPD)
      → gemma-4-26b-a4b-it  (Gemini, 30 RPM, 3K RPD)
```

### Audio chain (STT, голосовые)
```
gemini-3.1-flash-lite     (Gemini, 30 RPM, 3K RPD, 500K TPM)
  → whisper-large-v3-turbo (Groq, 40 RPM, 4K RPD)
    → whisper-large-v3     (Groq, 40 RPM, 4K RPD)
```

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`):
- Go тесты + vet
- Frontend сборка
- Docker build + push в ghcr.io

**Для работы push в ghcr.io:** Settings → Actions → General → Workflow permissions → `Read and write permissions`.

## Лицензия

MIT
