# Feature Guide — How to Add New Features to GroupBot

> Этот документ описывает процесс добавления нового функционала в бота:
> от инструментов агента до Telegram-команд, админки и бэкенда.

---

## 1. Определите тип фичи

| Тип | Пример | Бэкенд | Telegram | Admin UI |
|---|---|---|---|---|
| **Tool агента** | `get_schedule` | `internal/agent/tools.go` | @bot вызов | Нет |
| **Telegram-команда** | `/tag`, `/resend` | `cmd/bot/main.go` `handleSlashCommand` | Прямая команда | Нет |
| **Settings callback** | настройка Setka | `internal/handler/handler_settings.go` | `/settings` меню | Нет |
| **Webhook обработчик** | уведомления расписания | `internal/handler/handler_webhook.go` | Нет | Нет |
| **REST endpoint** | CRUD групп | `internal/api/` | Нет | React SPA |
| **Feature toggle** | вкл/выкл модуля | `internal/handler/handler_settings.go` + `internal/agent/orchestrator.go` | `/settings` + `features.json` | Admin UI |

---

## 2. Tool агента (рекомендуемый способ)

### 2.1 Определите инструмент в `internal/agent/tools.go`

```go
var AvailableTools = []llm.Tool{
    // ... существующие инструменты ...
    {
        Name:        "my_new_tool",
        Description: "Описание на русском (видят студенты и LLM). Синонимы: /команда1 /команда2",
        Parameters:  map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "arg1": map[string]interface{}{
                    "type": "string",
                    "description": "Описание аргумента",
                },
            },
            "required": []interface{}{"arg1"},
        },
    },
}
```

### 2.2 Реализуйте логику в `ToolExecutor`

```go
// В internal/agent/tools.go
func (e *ToolExecutor) executeMyNewTool(ctx context.Context, chatID int64, args map[string]interface{}) (string, error) {
    arg1 := getStringArg(args, "arg1")
    // ... ваша логика ...
    return "результат", nil
}
```

Добавьте в `Execute` switch-case:

```go
case "my_new_tool":
    return e.executeMyNewTool(ctx, chatID, parsed.Args)
```

### 2.3 Настройте права (опционально)

По умолчанию инструмент доступен всем. Чтобы ограничить админами:

**Вариант A — hardcoded** (быстро):
```go
var restrictedTools = map[string]bool{
    "my_new_tool": true,
    // ...
}
```

**Вариант B — через DB** (рекомендуется):
```sql
INSERT INTO command_permissions (group_id, command, allowed_role)
VALUES (?, 'my_new_tool', 'admin');
```

### 2.4 Добавьте feature toggle (опционально)

Если фича должна включаться/выключаться через настройки:

1. Добавьте ключ в `defaultFeatures()` в `internal/handler/handler_settings.go`:
   ```go
   func defaultFeatures() map[string]bool {
       return map[string]bool{
           "my_new_tool": true,
           // ...
       }
   }
   ```

2. Добавьте кнопку в Telegram-меню настроек:
   ```go
   // В handler_settings.go, метод showSettingsScreen()
   {Text: toggleLabel("Мой инструмент", features["my_new_tool"]),
    CallbackData: "toggle:my_new_tool"}
   ```

---

## 3. Telegram-команда

### 3.1 Зарегистрируйте команду

В `cmd/bot/main.go`:

```go
tgBot.SetMyCommands(context.Background(), &tgbot.SetMyCommandsParams{
    Commands: []models.BotCommand{
        // ... существующие команды ...
        {Command: "mycommand", Description: "описание команды"},
    },
    LanguageCode: "ru",
})
```

### 3.2 Добавьте handler

Три способа:

**A — `RegisterHandler`** (для точного совпадения):
```go
tgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/mycommand", tgbot.MatchTypeExact,
    func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
        // обработка
    })
```

**B — `handleSlashCommand` switch-case** (для сложной логики):
```go
case "mycommand":
    // логика команды
```

**C — `RegisterHandlerMatchFunc`** (для префиксов/сложных условий).

### 3.3 Добавьте сообщения в `messages.yaml`

```yaml
mycommand_response: "Результат команды: {{param}}"
```

---

## 4. REST API Endpoint

### 4.1 Создайте handler

```go
// internal/api/handler_myfeature.go
package api

import "github.com/gofiber/fiber/v2"

func (s *Server) handleMyFeature(c *fiber.Ctx) error {
    var req myFeatureRequest
    if err := c.BodyParser(&req); err != nil {
        return respondError(c, fiber.StatusBadRequest, ErrInvalidRequest, "invalid body")
    }
    // ... логика ...
    return respondSuccess(c, fiber.Map{"result": "ok"})
}
```

### 4.2 Зарегистрируйте роут

```go
// internal/api/router.go, в setupRoutes()
api.Get("/my-feature", s.handleMyFeature)
api.Post("/my-feature", s.handleMyFeature)
```

### 4.3 Добавьте в админку (React)

```tsx
// admin/src/pages/MyFeaturePage.tsx
// Или расширите существующую страницу
```

---

## 5. Feature toggle (вкл/выкл через настройки)

### 5.1 Бэкенд

Feature toggles хранятся в `data/groups/{chat_id}/features.json`.

Формат:
```json
{
  "my_new_tool": true,
  "enable_schedule": true,
  ...
}
```

**Важно:** ключи в features.json должны совпадать с `tool.Name` в `AvailableTools`.

### 5.2 В Telegram-меню

В `handler_settings.go`:

```go
// Добавить кнопку в showSettingsScreen()
{Text: toggleLabel("Мой инструмент", features["my_new_tool"]),
 CallbackData: "toggle:my_new_tool"}
```

```go
// Обработать toggle
case "toggle:my_new_tool":
    // инвертировать значение
```

### 5.3 В Admin UI

```tsx
// admin/src/components/FeaturesForm.tsx
<label>
  <input type="checkbox" checked={features.my_new_tool} />
  Мой инструмент
</label>
```

---

## 6. Новая таблица в БД

### 6.1 Добавьте `CREATE TABLE` в `internal/db/migrate.go`

```go
queries = append(queries,
    `CREATE TABLE IF NOT EXISTS my_new_table (
        id         INTEGER PRIMARY KEY,
        chat_id    INTEGER NOT NULL REFERENCES groups(chat_id) ON DELETE CASCADE,
        data       TEXT NOT NULL,
        created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    );`,
)
```

### 6.2 Добавьте методы в `internal/db/groups.go` (или новый файл)

```go
func (d *DB) CreateMyRecord(ctx context.Context, r MyRecord) error { ... }
func (d *DB) GetMyRecords(ctx context.Context, chatID int64) ([]MyRecord, error) { ... }
```

---

## 7. Повторное использование: чеклист

Перед добавлением фичи убедитесь, что ответили на вопросы:

- [ ] Инструмент агента или команда Telegram?
- [ ] Доступно всем или только админам?
- [ ] Должно ли быть настраиваемым (feature toggle)?
- [ ] Нужна ли новая таблица в SQLite?
- [ ] Нужен ли REST endpoint?
- [ ] Нужна ли кнопка в Telegram `/settings`?
- [ ] Нужен ли UI в React Admin?
- [ ] Нужна ли интеграция с Setka (вебхук)?
- [ ] Добавить ли сообщения в `messages.yaml`?

---

## 8. Структура пакетов (карта)

```
internal/
├── agent/            ← AgentOrchestrator, ToolExecutor, AvailableTools
│   └── tools.go      ← Все инструменты и их реализации
├── handler/          ← Telegram-хендлеры (message, mention, webhook, settings)
│   └── middleware.go  ← Rate-limit middleware
├── api/              ← Fiber REST API
│   ├── router.go     ← Регистрация всех роутов
│   ├── handler_*.go  ← Хендлеры по доменам
│   └── middleware_auth.go ← JWT авторизация
├── db/               ← SQLite + миграции + CRUD методы
│   ├── migrate.go
│   └── groups.go     ← Group + Superadmin + вспомогательные запросы
├── permissions/      ← command_permissions enforcement (P1#7)
│   └── service.go
├── schedule/         ← Diff engine, announcement, webhook payload types
│   └── diff.go
├── llm/              ← LLM client, provider chain, prompt registry
│   ├── client.go
│   ├── provider.go
│   ├── tracker.go
│   └── prompts.go
├── messages/         ← messages.yaml loader
├── config/           ← cleanenv config
└── util/             ← MessageSender, slug, date helpers
```

---

## 9. Webhook-контракт (omsu_setka → omsu_bot)

`omsu_setka` — [отдельный проект](https://github.com/pgeyko/omsu_setka), интеграция через HTTP webhook.

При изменении контракта вебхука:

1. **Setka**: измените `Payload` struct и `Notify()` в репозитории omsu_setka
2. **Bot** (`internal/schedule/diff.go`): измените `WebhookPayload` struct
3. **Bot** (`internal/handler/handler_webhook.go`): измените валидацию/обработку
4. **Spec**: обновите `dev/SPEC.md` и `GROUPBOT_TZ.md`
5. **Тесты контракта**: создайте или обновите интеграционный тест

---

## 10. После реализации

- [ ] `go vet ./...` — чисто
- [ ] `go test ./... -count=1` — все тесты проходят
- [ ] `dev/SPEC.md` — обновлён при изменении схемы/контрактов
- [ ] `dev/TASKS.md` — отмечено выполнением
- [ ] `messages.yaml` — добавлены новые фразы
