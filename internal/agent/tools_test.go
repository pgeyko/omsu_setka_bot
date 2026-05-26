package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"omsu_bot/internal/buffer"
	"omsu_bot/internal/db"
	"omsu_bot/internal/telegram"
)

func TestRunProtocol_DeleteMessagesAndCloseTopic(t *testing.T) {
	if os.Getenv("LLM_SKIP_FALLBACK_MODEL") == "" && os.Getenv("GROQ_API_KEY") == "" {
		t.Skip("skipping integration test: no LLM provider configured")
	}
	t.Parallel()
	// 1. Initialize in-memory database and migrate
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	ctx := context.Background()
	chatID := int64(12345)
	threadID := 1

	// Insert mock group and topic
	err = database.CreateGroup(ctx, &db.Group{
		ChatID:   chatID,
		Title:    "Test Group",
		APIToken: "token",
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("failed to insert group: %v", err)
	}

	_, err = database.ExecContext(ctx,
		`INSERT INTO topics (group_id, tg_thread_id, name, slug, aliases, description, hashtags, is_active)
		 VALUES (?, ?, 'General', 'general', '[]', '', '[]', 1)`, chatID, threadID)
	if err != nil {
		t.Fatalf("failed to insert topic: %v", err)
	}

	// 2. Set up SummaryBuffer
	buf := buffer.NewSummaryBuffer(database.DB, 10)

	// Push some messages
	buf.Push(chatID, threadID, 101, "alice", "hello")
	buf.Push(chatID, threadID, 102, "bob", "world")
	buf.Push(chatID, threadID, 103, "charlie", "testing protocol")
	buf.Push(chatID, threadID, 104, "dave", "keep this")

	// Verify messages are in DB and buffer (using retry loop since Push is async)
	var countBefore int
	for i := 0; i < 50; i++ {
		err = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM message_buffer WHERE chat_id = ? AND thread_id = ?", chatID, threadID).Scan(&countBefore)
		if err == nil && countBefore == 4 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if countBefore != 4 {
		t.Errorf("expected 4 messages in DB, got %d", countBefore)
	}

	// 3. Set up temporary protocols.json
	tempDir := t.TempDir()
	protoPath := filepath.Join(tempDir, "protocols.json")
	protoConfig := map[string]interface{}{
		"protocols": []map[string]interface{}{
			{
				"name": "зачистка",
				"actions": []map[string]interface{}{
					{
						"type":  "delete_messages",
						"count": 3,
					},
					{
						"type": "close_topic",
					},
				},
			},
		},
	}
	protoBytes, err := json.Marshal(protoConfig)
	if err != nil {
		t.Fatalf("failed to marshal proto config: %v", err)
	}
	if err := os.WriteFile(protoPath, protoBytes, 0644); err != nil {
		t.Fatalf("failed to write protocols.json: %v", err)
	}

	// 4. Create UsernameCache and ToolExecutor
	uc := telegram.NewUsernameCache()
	executor := &ToolExecutor{
		db:                  database.DB,
		bot:                 nil, // bot is nil, which skips Telegram API calls safely
		buffer:              buf,
		usernameCache:       uc,
		ProtocolsConfigPath: protoPath,
	}

	// 5. Execute run_protocol
	args := `{"protocol_name": "зачистка", "thread_id": "1"}`
	res, err := executor.Execute(ctx, chatID, "run_protocol", args)
	if err != nil {
		t.Fatalf("failed to execute run_protocol: %v", err)
	}

	// Verify result output
	if !strings.Contains(res, "Протокол 'зачистка' выполнен") {
		t.Errorf("unexpected output: %s", res)
	}
	if !strings.Contains(res, "Удалено сообщений из Telegram и базы данных: 3") {
		t.Errorf("expected deletion of 3 messages in result, got: %s", res)
	}
	if !strings.Contains(res, "Топик успешно закрыт") {
		t.Errorf("expected topic close confirmation in result, got: %s", res)
	}

	// Verify database changes
	var countAfter int
	err = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM message_buffer WHERE chat_id = ? AND thread_id = ?", chatID, threadID).Scan(&countAfter)
	if err != nil {
		t.Fatalf("failed to query message_buffer: %v", err)
	}
	if countAfter != 1 {
		t.Errorf("expected only 1 message left in DB, got %d", countAfter)
	}

	// Check if the remaining message is indeed "hello" (which is the oldest message, since delete deletes recent DESC)
	var remainingText string
	var remainingID int
	err = database.QueryRowContext(ctx, "SELECT message_id, text FROM message_buffer WHERE chat_id = ? AND thread_id = ?", chatID, threadID).Scan(&remainingID, &remainingText)
	if err != nil {
		t.Fatalf("failed to query remaining message: %v", err)
	}
	if remainingID != 101 || remainingText != "hello" {
		t.Errorf("expected remaining message to be 101/hello, got %d/%s", remainingID, remainingText)
	}

	// Verify memory buffer changes
	msgs := buf.GetMessages(chatID, threadID)
	if !strings.Contains(msgs, "hello") {
		t.Errorf("expected memory buffer to retain 'hello', got: %s", msgs)
	}
	if strings.Contains(msgs, "world") || strings.Contains(msgs, "testing protocol") || strings.Contains(msgs, "keep this") {
		t.Errorf("expected recent messages to be removed from memory buffer, got: %s", msgs)
	}

	// Verify topic status in database
	var isActive int
	err = database.QueryRowContext(ctx, "SELECT is_active FROM topics WHERE group_id = ? AND tg_thread_id = ?", chatID, threadID).Scan(&isActive)
	if err != nil {
		t.Fatalf("failed to query topic status: %v", err)
	}
	if isActive != 0 {
		t.Errorf("expected topic is_active to be 0, got %d", isActive)
	}
}

func TestRunProtocol_ModerateUser(t *testing.T) {
	t.Parallel()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	ctx := context.Background()
	chatID := int64(12345)

	// Set up temporary protocols.json
	tempDir := t.TempDir()
	protoPath := filepath.Join(tempDir, "protocols.json")
	protoConfig := map[string]interface{}{
		"protocols": []map[string]interface{}{
			{
				"name": "мут_хулигана",
				"actions": []map[string]interface{}{
					{
						"type":             "mute_user",
						"duration_minutes": 15,
					},
				},
			},
		},
	}
	protoBytes, err := json.Marshal(protoConfig)
	if err != nil {
		t.Fatalf("failed to marshal proto config: %v", err)
	}
	if err := os.WriteFile(protoPath, protoBytes, 0644); err != nil {
		t.Fatalf("failed to write protocols.json: %v", err)
	}

	// Create UsernameCache and populate it
	uc := telegram.NewUsernameCache()
	uc.Store("troublemaker", 999)

	buf := buffer.NewSummaryBuffer(database.DB, 10)
	executor := &ToolExecutor{
		db:                  database.DB,
		bot:                 nil,
		buffer:              buf,
		usernameCache:       uc,
		ProtocolsConfigPath: protoPath,
	}

	// 1. Success case
	argsSuccess := `{"protocol_name": "мут_хулигана", "username": "@troublemaker"}`
	res, err := executor.Execute(ctx, chatID, "run_protocol", argsSuccess)
	if err != nil {
		t.Fatalf("failed to execute run_protocol: %v", err)
	}
	if !strings.Contains(res, "Пользователь @troublemaker замучен на 15 минут.") {
		t.Errorf("expected mute log, got: %s", res)
	}

	// 2. User not found in cache
	argsNotFound := `{"protocol_name": "мут_хулигана", "username": "@unknown_user"}`
	resNotFound, err := executor.Execute(ctx, chatID, "run_protocol", argsNotFound)
	if err != nil {
		t.Fatalf("failed to execute run_protocol: %v", err)
	}
	if !strings.Contains(resNotFound, "Ошибка: пользователь @unknown_user не найден в кэше.") {
		t.Errorf("expected not found error, got: %s", resNotFound)
	}
}

func TestRunProtocol_ProtocolNotFound(t *testing.T) {
	t.Parallel()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	tempDir := t.TempDir()
	protoPath := filepath.Join(tempDir, "protocols.json")
	protoConfig := map[string]interface{}{
		"protocols": []map[string]interface{}{},
	}
	protoBytes, _ := json.Marshal(protoConfig)
	_ = os.WriteFile(protoPath, protoBytes, 0644)

	executor := &ToolExecutor{
		db:                  database.DB,
		bot:                 nil,
		buffer:              buffer.NewSummaryBuffer(database.DB, 10),
		usernameCache:       telegram.NewUsernameCache(),
		ProtocolsConfigPath: protoPath,
	}

	args := `{"protocol_name": "неизвестный"}`
	res, err := executor.Execute(context.Background(), 12345, "run_protocol", args)
	if err != nil {
		t.Fatalf("failed to execute run_protocol: %v", err)
	}
	if !strings.Contains(res, "Протокол 'неизвестный' не найден в конфигурации.") {
		t.Errorf("expected not found message, got: %s", res)
	}
}
