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

- Go 1.23+
- Node.js 24 (для сборки админки)
- Токены: Gemini API, DeepSeek API (опционально), Telegram Bot

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
docker-compose up -d
```

Для интеграции с расписанием требуется отдельно запущенный [omsu_setka](https://github.com/pgeyko/omsu_setka).

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
├── prompts/                  # LLM-промпты
├── admin/                    # React SPA админка
├── config.yaml               # Конфигурация
├── persona.md                # Seed-файл персоны
├── messages.yaml             # Тексты ответов
├── protocols.json            # Протоколы агента
└── Dockerfile
```

## LLM Provider Chain

### Text Chain (classify, agent, summary)
```
gemma-4-31b-it       (15 RPM, 1500 TPM)
 → gemma-4-26b-it    (15 RPM, 1500 TPM)
  → gemini-3.1-flash-lite (15 RPM, 500 RPD)
   → deepseek-chat
```

### Vision + Audio STT Chain
```
gemma-4-31b-it       (15 RPM, 1500 TPM)
 → gemini-3.1-flash-lite (15 RPM, 500 RPD)
```

## CI/CD

GitHub Actions (`.github/workflows/ci.yml`):
- Go тесты + vet
- Frontend сборка
- Docker build + push в ghcr.io

**Для работы push в ghcr.io:** Settings → Actions → General → Workflow permissions → `Read and write permissions`.

## Лицензия

MIT
