# GitHub Secrets — omsu_bot (GroupBot)

Переменные, которые нужно добавить в `Settings → Secrets and variables → Actions` репозитория [pgeyko/omsu_bot](https://github.com/pgeyko/omsu_bot).

---

## Обязательные

| Secret | Описание | Пример |
|---|---|---|
| `SERVER_HOST` | IP или домен сервера для деплоя | `95.xxx.xxx.xxx` или `bot.example.com` |
| `SERVER_USER` | Пользователь для SSH | `deploy` |
| `SSH_PRIVATE_KEY` | Приватный SSH-ключ (без пароля) | `-----BEGIN OPENSSH PRIVATE KEY-----\n...` |
| `BOT_TOKEN` | Токен Telegram бота от @BotFather | `8181690472:AAHjXuO...` |
| `GROUP_ID` | ID супергруппы (с минусом) | `-1003990812833` |
| `GROQ_API_KEY` | API ключ Groq (primary) | `gsk_your_groq_key` |
| `GEMINI_API_KEY` | API ключ Gemini | `AIzaSyChTGHR3...` |
| `ADMIN_SECRET` | Секрет для получения JWT | `my-secret-admin-key` |
| `JWT_SECRET` | Секрет подписи JWT (любая строка) | `my-jwt-signing-secret` |
| `SCHEDULE_WEBHOOK_SECRET` | HMAC-ключ подписи вебхуков от omsu_setka | `webhook-secret-123` |

---

## Опциональные

| Secret | По умолчанию | Описание |
|---|---|---|
| `SSH_PORT` | `22` | Порт SSH |
| `DEPLOY_PATH` | `~/omsu_setka_bot` | Путь к проекту на сервере |
| `OMSU_GROUP_ID` | — | ID группы в справочнике ОмГУ (для интеграции с omsu_setka) |
| `DEEPSEEK_API_KEY` | — | API ключ DeepSeek (резервный LLM провайдер) |
| `GROQ_API_KEY_2` | — | Резервный ключ Groq (второй аккаунт) — когда primary выбирает TPD/TPM |
| `OPENROUTER_API_KEY` | — | Ключ OpenRouter — last-resort fallback (gpt-oss-120b:free / gpt-oss-20b:free / openrouter/free catch-all) |
| `GEMINI_RESERVE_API_KEY` | — | Резервный ключ Gemini (второй аккаунт) |
| `SETKA_BASE_URL` | — | URL omsu_setka API. Если оба контейнера на одной Docker-сети: `http://setka_backend:8080` |
| `SETKA_ADMIN_KEY` | — | Admin-ключ для регистрации вебхука в omsu_setka |
| `SETKA_PUBLIC_URL` | — | Публичный URL фронтенда расписания (для ссылок в ответах бота). Например `https://setka.pgeyko.ru` |

---

## Pipeline

| Джоба | Условие | Действие |
|---|---|---|
| `backend-test` | PR / push | `go test ./...` |
| `frontend-test` | PR / push | `npm ci` + `npm run build` |
| `build-and-push` | push в main/master | `docker build` → `ghcr.io/pgeyko/omsu_setka_bot/bot:{latest,sha}` |
| `deploy` | push в main/master | SCP `docker-compose.prod.yml` → SSH → `docker compose up -d` |

Образ пушится в `ghcr.io/pgeyko/omsu_setka_bot/bot`.
