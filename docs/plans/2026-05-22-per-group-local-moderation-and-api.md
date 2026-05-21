# Per-Group Local Moderation and Groups API Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement per-group local moderation configuration (enable/disable math captcha, link filter, and flood control) via settings and the REST API. Expose groups list and detail endpoints in the API. Fully integrate settings toggles and detail views in the React Admin panel, and update Swagger documentation for the missing endpoints.

**Architecture:** Extend the group features defaults mapping to support `enable_captcha`, `enable_link_filter`, and `enable_flood_control`. Integrate the features check into `handlers.Antispam` so checks are skipped if disabled. Enhance the REST API with group-specific contexts, and update the React Admin groups page to include a designated "Локальная модерация" section. Update Swagger annotations and generate Swagger documentation.

**Tech Stack:** Go 1.22+, SQLite, Fiber (API), React, Tailwind/CSS.

---

## Task 1: Go Backend - Local Moderation Features & Antispam Checks

**Files:**
- Modify: [handler_settings.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/handler/handler_settings.go)
- Modify: [context_handler.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/api/context_handler.go)
- Modify: [antispam.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/handler/antispam.go)
- Modify: [main.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/cmd/bot/main.go)
- Test: [antispam_test.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/handler/antispam_test.go)

**Step 1: Standardize Default Features Loader**
Implement a shared helper `defaultFeatures()` in `handler_settings.go` and `context_handler.go` to include:
- `enable_captcha`: true
- `enable_link_filter`: true
- `enable_flood_control`: true
Ensure `LoadFeatures` fills missing fields with these defaults.

**Step 2: Update Telegram Settings Keyboard**
Add `enable_captcha`, `enable_link_filter`, and `enable_flood_control` buttons to `showToolsScreen` inside `handler_settings.go`.

**Step 3: Refactor Antispam to Support Features Loader**
Modify `NewAntispam` signature to accept a features loader:
```go
func NewAntispam(loaders ...func(chatID int64) map[string]bool) *Antispam
```
Inside `HandleNewChatMembers`, `CheckFloodAndLinks`, and `HandleCallbackQuery`, load features and bypass check logic if the corresponding flag or `enable_moderation` is disabled.

**Step 4: Update main.go and Verify Tests**
Update `NewAntispam` instantiation in `main.go` to pass settings features loader. Run `go test ./...` to verify Go tests pass.

---

## Task 2: React Admin Dashboard - Group Details and Moderation Section

**Files:**
- Modify: [GroupsPage.tsx](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/admin/src/pages/GroupsPage.tsx)

**Step 1: Update featuresForm State Defaults**
Add `enable_captcha`, `enable_link_filter`, and `enable_flood_control` (default `true`) to `featuresForm` state.

**Step 2: Update React Render Layout**
Reorganize group features into two clear, aesthetic sections:
- "Основные модули" (Schedule, Summary, Voice, Photo)
- "Локальная модерация" (Moderation, Captcha, Link Filter, Flood Control)

**Step 3: Validate & Build React App**
Run `npm run build` inside `admin/` to verify TypeScript type checking and bundling compiles correctly.

---

## Task 3: Swagger Documentation & Route Stubs

**Files:**
- Modify: [swagger_docs.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/api/swagger_docs.go)
- Modify: [handler_auth.go](file:///home/pwlgk/Development/ai_isolated/omsu/omsu_bot/internal/api/handler_auth.go)

**Step 1: Add Stub Functions to swagger_docs.go**
Ensure every comment block in `swagger_docs.go` has a trailing dummy function:
```go
func _get_persona_stub() {}
```

**Step 2: Document Missing Endpoints**
Add Swagger comments and trailing stubs in `swagger_docs.go` for:
- `GET /api/groups`
- `POST /api/groups`
- `GET /api/groups/{chat_id}`
- `PUT /api/groups/{chat_id}`
- `DELETE /api/groups/{chat_id}`
- `GET /api/groups/{chat_id}/context/persona`
- `PUT /api/groups/{chat_id}/context/persona`
- `GET /api/groups/{chat_id}/context/system-prompt`
- `PUT /api/groups/{chat_id}/context/system-prompt`
- `GET /api/groups/{chat_id}/context/knowledge`
- `PUT /api/groups/{chat_id}/context/knowledge`
- `GET /api/groups/{chat_id}/context/features`
- `PUT /api/groups/{chat_id}/context/features`
- `GET /api/admin/superadmins`
- `POST /api/admin/superadmins`
- `DELETE /api/admin/superadmins/{user_id}`
- `POST /api/admin/groups/register-webhooks`

**Step 3: Generate Swagger API Docs**
Run:
```bash
go run github.com/swaggo/swag/cmd/swag@v1.16.3 init -g main.go -d cmd/bot,internal/api
```
Verify `docs/docs.go` is generated with all paths correctly documented.
