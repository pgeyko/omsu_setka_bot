# omsu_bot — CLAUDE.md

This file is the Claude-specific entrypoint for the `omsu_bot` subproject.

Read in this order:
1. This file (Claude adapter)
2. `AGENTS.md` (canonical bot instructions)
3. `../AGENTS.md` (workspace root rules)
4. `.agents/README.md` (skill registry)
5. Relevant skills from `.agents/skills/`

## Claude-specific notes

- Use the `view_file` tool to read skill files before coding.
- After every code edit, run `go vet ./...` and `go test ./...`.
- Mark tasks in `dev/TASKS.md` as `[/]` before starting and `[x]` after finishing.
- Never commit without reading `.agents/skills/commit.md` first.
