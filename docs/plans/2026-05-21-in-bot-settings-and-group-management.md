# In-Bot Admin Settings Panel & Superadmin API Extensions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement an interactive `/settings` command inside Telegram for group admins to configure their group's knowledge base, persona/prompt, features, and schedule ID (via search) without overloading the LLM context. Extend superadmin REST API endpoints for managing superadmins and webhooks, and ensure the bot handles topicless groups correctly.

**Architecture:** Use a stateful `SessionStore` inside `omsu_bot` to capture admin text responses (KB write, schedule search). Retrieve Setka groups using the search REST API and return select buttons to the admin. For topicless groups, bypass topic classification/forwarding while maintaining general chat summaries, mentions, and commands.

**Tech Stack:** Go 1.21+, SQLite, Fiber (API), go-telegram/bot (Telegram API).

---

## Task 1: Session Store and Group Settings Handler

**Files:**
- Create: [session.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/telegram/session.go)
- Create: [handler_settings.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/handler/handler_settings.go)
- Test: [session_test.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/telegram/session_test.go)

**Step 1: Write the SessionStore**
Create `session.go` and `session_test.go` checking basic states: `StateNone`, `StateWaitingForKB`, `StateWaitingForPersona`, `StateWaitingForPrompt`, `StateWaitingForScheduleSearch`.

**Step 2: Write Settings Command and Callback Menu**
Implement the `/settings` command and `HandleCallbackQuery` in `handler_settings.go` with security checks validation via `AdminCache.IsAdmin`.

---

## Task 2: Step-by-Step Settings Input and Setka Search

**Files:**
- Modify: [handler_settings.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/handler/handler_settings.go)
- Modify: [groups.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/db/groups.go)

**Step 1: Implement Input Saving**
Write `HandleAdminInput` to process raw text messages from admin depending on their session state (saving `persona.md`, `system_prompt.txt`, `knowledge_base.txt`).

**Step 2: Implement Setka Search & Selection**
Make HTTP request to Setka API inside `HandleAdminInput` for `StateWaitingForScheduleSearch` to lookup group IDs. Present results as inline buttons and save selection to database.

---

## Task 3: Superadmin API Extensions and Topicless Groups Bypass

**Files:**
- Modify: [context_handler.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/api/context_handler.go)
- Modify: [router.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/api/router.go)
- Modify: [handler_message.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/handler/handler_message.go)

**Step 1: Add API endpoints**
Expose `GET/POST/DELETE /api/admin/superadmins` and `POST /api/admin/groups/register-webhooks`.

**Step 2: Bypassing Classifier**
Skip classification/forwarding in `Handler.HandleMessage` if the group has 0 active topics in the DB.

---

## Task 4: Main Integration and Final Tests

**Files:**
- Modify: [main.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/cmd/bot/main.go)
- Test: [context_handler_test.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/api/context_handler_test.go)

**Step 1: Register Handlers and Setup Main Wiring**
Wire `SessionStore`, `/settings` callbacks, and input redirections in `main.go`.

**Step 2: Verification**
Run `go test ./...` to verify all components compile and tests pass.
