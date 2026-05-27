# Agent Instructions — omsu_bot (GroupBot)

Read the workspace `AGENTS.md` at `../AGENTS.md` first, then follow the
rules in this file. This file is the canonical entrypoint for agents working
on the `omsu_bot` project.

## Project Overview

GroupBot is a Telegram bot for student groups (multi-tenant — one process serves
multiple groups). It lives in forum supergroups (topics) and does three things:

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
├── cmd/bot/
│   ├── main.go            # Entry point — config, init, signal handling
│   ├── commands.go        # CommandHandler type + registry
│   ├── commands_init.go   # /init, /help, /start, /status
│   ├── commands_topics.go # /register, /topics, /id
│   ├── commands_misc.go   # /resend, /tag, /settings, /summary
│   ├── helpers.go         # setupLogger, isBotCommand, isBotMention
│   └── slash.go           # handleSlashCommand dispatch
├── internal/
│   ├── agent/             # Agent orchestrator + tool executor (map-based dispatch)
│   ├── app/               # InitDB, SetupLogger, InitPersona, StartSighupHandler
│   ├── handler/           # Telegram handlers (message, mention, webhook, settings, antispam)
│   ├── classifier/        # LLM classification logic
│   ├── forwarder/         # Message duplication
│   ├── schedule/          # Diff engine + LLM announcer + BotPoster
│   ├── persona/           # Bot persona store (name + system_prompt)
│   ├── api/               # Fiber REST admin API
│   ├── llm/               # LLM provider chain
│   ├── circuitbreaker/    # Circuit breaker (trip→cooldown→reset)
│   ├── db/                # SQLite + migrations + repositories
│   │   ├── groups.go      # Group CRUD
│   │   ├── topics_repo.go # Topic queries (GetBySlug, Create, SearchByPrefix, …)
│   │   ├── processed_repo.go # Processed message dedup (INSERT OR IGNORE)
│   │   ├── media_repo.go  # Media group items
│   │   ├── tags.go        # Hashtag search
│   │   ├── config.go      # bot_config key-value
│   │   └── migrate.go     # Schema migrations
│   ├── buffer/            # Summary message buffer
│   ├── media/             # Photo OCR + Voice STT
│   ├── messages/          # messages.yaml loader
│   ├── permissions/       # Command permission checks
│   ├── skills/            # YAML-based skill registry
│   ├── telegram/          # Admin cache, session store, username cache, webhook sync
│   ├── config/            # cleanenv config
│   └── util/              # Date/slug/truncate helpers
├── prompts/               # LLM prompt templates (*.txt, *.md)
├── skills/                # LLM-tool YAML definitions
│   ├── manifest.yaml      # Skill → handler mapping
│   ├── get_schedule.yaml
│   ├── forward_message.yaml
│   ├── generate_summary.yaml
│   ├── manage_topic.yaml
│   ├── moderate_user.yaml
│   └── run_protocol.yaml
├── admin/                 # React SPA (admin panel)
├── persona.md             # Seed file for bot persona (first-run)
├── config.yaml            # Runtime configuration
├── messages.yaml          # Bot response messages
├── protocols.json         # Moderation action protocols
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
- Config via `github.com/ilyakaznacheev/cleanenv` (env + YAML).
- JWT via `github.com/golang-jwt/jwt/v5`.
- Telegram via `github.com/go-telegram/bot`.
- LLM via plain `net/http` — no SDK. Three providers: Groq (primary, OpenAI-compatible) + Gemini (fallback) + OpenRouter (last-resort free tier).
- Four specialized chains: agent (llama-3.3-70b → qwen3-32b → gemma-31b → gemma-26b → openai/gpt-oss-120b:free → openrouter/free), simple (llama-3.1-8b-instant → qwen3-32b → gemini-3.1-flash-lite → gpt-oss-20b:free → openrouter/free), vision (scout-17b → flash-lite → gemma-31b → gemma-26b), audio (flash-lite → whisper-turbo → whisper-v3).
- All SQL must be parameterized — no string formatting for queries.
- SQL is organized in repository methods under `internal/db/` (topics_repo, processed_repo, media_repo).
- ToolExecutor uses `map[string]ToolFunc` dispatch (not switch). New tool = add entry to map + implement method.
- Slash commands use `CommandHandler` registry with `init()` registration (not switch). New command = register in `init()` + implement handler.
- No global `var app` — package-level vars (botUsername, globalLLM, providerCount) used instead.
- Persona system prompt injected as `system` role in every LLM call.
- Rate limit: 10 LLM requests / min / user (in-memory, per user_id).
- HMAC-SHA256 on webhook endpoint (`X-Webhook-Signature` header).

## SQLite Schema (core tables)

```sql
groups (chat_id PK, title, api_token, omsu_group_id, is_active, is_vip, created_at)
superadmins (user_id PK, note, created_at)
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
