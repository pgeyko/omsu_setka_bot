# Agent Instructions — omsu_bot (GroupBot)

Read the workspace `AGENTS.md` at `../AGENTS.md` first, then follow the
rules in this file. This file is the canonical entrypoint for agents working
inside the `omsu_bot/` subproject.

## Project Overview

GroupBot is a Telegram bot for a student group. One process = one group.
It lives in a forum supergroup (topics) and does three things:

1. **Auto-forwards** messages to relevant topics using LLM classification.
2. **Announces** schedule changes received via webhook from omsu_setka.
3. **Has a configurable persona** — name and system prompt stored in SQLite,
   managed via Admin REST API without redeploying.

Full specification: `../GROUPBOT_TZ.md`

## Instruction Precedence

1. Agent runtime / tool instructions.
2. Task-specific user instructions.
3. This file (`omsu_bot/AGENTS.md`).
4. `../AGENTS.md` (workspace root).
5. `.agents/README.md` and `.agents/manifest.json`.
6. Relevant `.agents/skills/*.md`.

## Mandatory Skill Usage — NON-NEGOTIABLE

**Skills MUST be loaded before any code is written.** No exceptions.

| Task type | Required skills |
|---|---|
| Any code edit | `lint-and-validate` |
| Any git commit | `commit` |
| Go source file | `golang-pro` + `sql-pro` |
| REST API handler | `api-endpoint-builder` + `api-documentation` |
| Docker / Dockerfile | `docker-expert` + `bash-pro` |
| Performance work | `performance-optimizer` |
| Writing a plan | `writing-plans` |
| Multi-session work | `blueprint` |

### Loading protocol (mandatory):

```
1. Identify task type from the table above.
2. Read each required skill file IN FULL before touching code.
3. Apply guidance from skills throughout implementation.
4. After every code change: run lint-and-validate checks.
5. Before every commit: read the commit skill.
```

Skills are in `.agents/skills/` (relative to this directory).

## Task Tracking Protocol

Tasks live in `dev/TASKS.md`. Use this notation strictly:

| Marker | Meaning |
|---|---|
| `[ ]` | Not started |
| `[/]` | In progress — mark BEFORE starting |
| `[x]` | Done — mark AFTER completing, add one-line note |
| `[!]` | Blocked — document blocker inline |

**After completing an entire stage:**
Add `✅ Completed: YYYY-MM-DD` on the line below the stage heading.

**Never start Stage N+1 before all tasks in Stage N are `[x]`.**

## Architecture

```
omsu_bot/
├── cmd/bot/main.go
├── internal/
│   ├── agent/            # Agent orchestrator + tool executor
│   ├── handler/          # Telegram handlers (message, mention, webhook, settings, antispam)
│   ├── classifier/       # LLM classification logic
│   ├── forwarder/        # Message duplication
│   ├── schedule/         # Diff engine + LLM announcer
│   ├── persona/          # Bot persona store (name + system_prompt)
│   ├── api/              # Fiber REST admin API
│   ├── llm/              # LLM provider chain + circuit breaker
│   ├── db/               # SQLite + migrations
│   ├── buffer/           # Summary message buffer
│   ├── media/            # Photo OCR + Voice STT
│   ├── messages/         # messages.yaml loader
│   ├── telegram/         # Admin cache, session store, username cache
│   ├── config/           # cleanenv config
│   └── util/             # Date/slug helpers
├── prompts/              # LLM prompt templates (*.txt)
├── persona.md            # Seed file for bot persona (first-run)
├── config.yaml           # Runtime configuration
├── messages.yaml         # Bot response messages
├── protocols.json        # Moderation action protocols
└── Dockerfile
```

## Development Commands

Prerequisites: Go 1.22+, Docker.

```bash
# Run locally
go run ./cmd/bot/main.go

# Run tests
go test ./...

# Build binary
go build -o groupbot ./cmd/bot

# Docker build
docker build -t groupbot .

# Docker Compose
docker-compose up --build
```

Validation after every change:

```bash
cd omsu_bot
go vet ./...
go test ./...
```

If tests cannot run due to missing deps, report the exact blocker.

## Backend Conventions

- Use `log/slog` (stdlib) for structured logging — not zerolog (that's setka).
- Use `github.com/gofiber/fiber/v2` for REST API.
- Use `modernc.org/sqlite` (pure Go, no CGO).
- Use `github.com/jmoiron/sqlx` for SQL queries.
- Use `golang-migrate/migrate` for migrations.
- Config via `github.com/ilyakaznacheev/cleanenv` (env + YAML).
- JWT via `github.com/golang-jwt/jwt/v5`.
- Telegram via `github.com/go-telegram/bot`.
- LLM via plain `net/http` — no SDK.
- All SQL must be parameterized — no string formatting for queries.
- Persona system prompt injected as `system` role in every LLM call.
- Rate limit: 5 LLM requests / min / user (in-memory, per user_id).
- HMAC-SHA256 on webhook endpoint (`X-Webhook-Signature` header).

## SQLite Schema (core tables)

```sql
groups (chat_id PK, title, api_token, omsu_group_id, is_active, is_vip, created_at)
superadmins (user_id PK, note, created_at)
bot_persona (id=1 singleton, name, system_prompt, signature, updated_at)
topics (id, group_id FK, tg_thread_id, name, slug, aliases JSON, description, hashtags JSON, is_active)
processed_messages (message_id, chat_id, thread_id, action, target_thread_id)
llm_requests (id, group_id FK, type, provider, input_tokens, output_tokens, model, cost_usd, created_at)
message_buffer (chat_id, thread_id, message_id, username, text, created_at)
message_tags (id, chat_id, message_id, tag, created_at)
schedule_snapshots (id, group_id, data JSON, created_at)
schedule_anomalies (id, snapshot_id FK, type, details JSON, notified, created_at)
command_permissions (group_id, command PK, allowed_role)
summary_requests (user_id, chat_id, requested_at)
bot_config (key PK, value)
revoked_tokens (jti PK, expires_at)
```

## Key Files

| File | Purpose |
|---|---|
| `../GROUPBOT_TZ.md` | Full functional specification |
| `dev/TASKS.md` | Staged task checklist — source of truth for progress |
| `dev/SPEC.md` | Technical reference (DB schema, API contracts, flows) |
| `persona.md` | Bot persona seed (name + system_prompt markdown) |
| `config.yaml` | Runtime config template |
| `.agents/skills/` | Local copy of required skills |
