# Shared Agent Registry — omsu_bot

`.agents/` содержит скиллы только для backend (Go) разработки.
Frontend-скиллы для React-админки (`admin/`) не входят в этот набор.

## Entry Points

- **Codex**: read `AGENTS.md`.
- **Claude**: read `CLAUDE.md` → `AGENTS.md`.
- **Gemini**: read `GEMINI.md` → `AGENTS.md`.

## Skill Groups

| Group | Skills |
|---|---|
| Backend | `golang-pro`, `sql-pro`, `api-endpoint-builder`, `api-documentation`, `performance-optimizer` |
| Infrastructure | `docker-expert`, `bash-pro` |
| Standards | `lint-and-validate` *(обязателен после каждого изменения кода)*, `writing-plans`, `blueprint`, `commit` *(обязателен перед каждым коммитом)* |

## Loading Rules

1. Identify task type → see table in `AGENTS.md`.
2. Read each required skill **in full** before writing code.
3. Run `lint-and-validate` after **every** code change.
4. Read `commit` before **every** commit.
