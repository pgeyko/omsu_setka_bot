package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	_ "image/gif"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"omsu_bot/internal/buffer"
	"omsu_bot/internal/classifier"
	"omsu_bot/internal/db"
	"omsu_bot/internal/forwarder"
	"omsu_bot/internal/telegram"
	"omsu_bot/internal/util"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Handler struct {
	classifier    *classifier.Classifier
	forwarder     *forwarder.Forwarder
	db            *sql.DB
	buffer        *buffer.SummaryBuffer
	usernameCache *telegram.UsernameCache
	bot           *tgbot.Bot
	token         string

	pendingMediaCancel map[string]context.CancelFunc
	pendingMu          sync.Mutex
}

func NewHandler(classifier *classifier.Classifier, forwarder *forwarder.Forwarder, bot *tgbot.Bot, token string, db *sql.DB, buffer *buffer.SummaryBuffer, usernameCache *telegram.UsernameCache) *Handler {
	return &Handler{
		classifier:         classifier,
		forwarder:          forwarder,
		db:                 db,
		buffer:             buffer,
		usernameCache:      usernameCache,
		bot:                bot,
		token:              token,
		pendingMediaCancel: make(map[string]context.CancelFunc),
	}
}

func (h *Handler) HandleMessage(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	if h.usernameCache != nil && msg.From != nil {
		h.usernameCache.Store(msg.From.Username, msg.From.ID)
	}

	var active int
	if err := h.db.QueryRowContext(ctx, "SELECT is_active FROM groups WHERE chat_id = ?", msg.Chat.ID).Scan(&active); err != nil || active != 1 {
		return
	}

	// Intercept Topic Creation and Edit
	if msg.ForumTopicCreated != nil {
		slug := "topic_" + fmt.Sprint(msg.MessageThreadID)
		_, err := h.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO topics (group_id, tg_thread_id, name, slug, description, is_active) VALUES (?, ?, ?, ?, ?, 1)`,
			msg.Chat.ID, msg.MessageThreadID, msg.ForumTopicCreated.Name, slug, "",
		)
		if err != nil {
			slog.Error("failed to insert forum topic created", "error", err)
		} else {
			slog.Info("registered new forum topic", "chat_id", msg.Chat.ID, "thread_id", msg.MessageThreadID, "name", msg.ForumTopicCreated.Name)
		}
		return
	}

	if msg.ForumTopicEdited != nil {
		if msg.ForumTopicEdited.Name != "" {
			_, err := h.db.ExecContext(ctx,
				`UPDATE topics SET name = ? WHERE group_id = ? AND tg_thread_id = ?`,
				msg.ForumTopicEdited.Name, msg.Chat.ID, msg.MessageThreadID,
			)
			if err != nil {
				slog.Error("failed to update forum topic edited", "error", err)
			} else {
				slog.Info("updated forum topic name", "chat_id", msg.Chat.ID, "thread_id", msg.MessageThreadID, "name", msg.ForumTopicEdited.Name)
			}
		}
		return
	}

	slog.Debug("tg message",
		"msg_id", msg.ID,
		"from", msg.From.ID,
		"username", msg.From.Username,
		"text", truncate(msg.Text, 500),
		"chat", msg.Chat.ID,
		"thread", msg.MessageThreadID,
	)

	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	// For documents without caption, use filename as text for classification
	if msg.Document != nil && text == "" && msg.Document.FileName != "" {
		text = msg.Document.FileName
	} else if msg.Document != nil && msg.Document.FileName != "" {
		text = text + " [файл: " + msg.Document.FileName + "]"
	}
	if h.buffer != nil && text != "" {
		h.buffer.Push(msg.Chat.ID, msg.MessageThreadID, msg.ID, msg.From.Username, text)
	}

	if h.isProcessed(ctx, msg.ID, msg.Chat.ID) {
		return
	}

	var activeTopicsCount int
	err := h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM topics WHERE group_id = ? AND is_active = 1", msg.Chat.ID).Scan(&activeTopicsCount)
	if err != nil || activeTopicsCount == 0 {
		h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "skipped_no_topics", 0)
		return
	}

	if !h.prefilter(msg) {
		h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "skipped", 0)
		return
	}

	var fileID string
	if len(msg.Photo) > 0 {
		fileID = msg.Photo[len(msg.Photo)-1].FileID
	} else if msg.Document != nil {
		fileID = msg.Document.FileID
	}

	if text == "" && fileID == "" {
		h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "skipped", 0)
		return
	}

	// Defer classification for media groups until all photos arrive
	if msg.MediaGroupID != "" {
		h.deferMediaGroup(ctx, b, msg, text, fileID)
		return
	}

	result, err := h.classifyMsg(ctx, msg.Chat.ID, text, fileID)
	if err != nil {
		slog.Warn("classification skipped", "reason", err, "msg_id", msg.ID)
		h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "skipped", 0)
		return
	}

	if result.Confidence < 0.75 || result.Topic == "" {
		h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "low_confidence", 0)
		return
	}

	targetThreadID, err := h.findOrCreateTopic(ctx, msg, result.Topic, result.Confidence)
	if err != nil || targetThreadID == 0 {
		slog.Warn("target topic not found", "topic", result.Topic, "confidence", result.Confidence)
		return
	}

if targetThreadID == msg.MessageThreadID {
	h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "skipped_same_topic", 0)
	return
}

	fromTopicName := ""
	if msg.MessageThreadID != 0 {
		fromTopicName = h.getTopicName(ctx, msg.Chat.ID, msg.MessageThreadID)
	}

	// Text-prefix deduplication: skip if same text already in target topic
	if text != "" {
		textPrefix := textPrefix(text)
		d := &db.DB{DB: h.db}
		exists, err := d.HasMessageWithText(ctx, msg.Chat.ID, targetThreadID, textPrefix)
		if err == nil && exists {
			slog.Debug("duplicate message detected, skipping forward", "msg_id", msg.ID, "topic", result.Topic)
			h.bot.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:          msg.Chat.ID,
				MessageThreadID: msg.MessageThreadID,
				Text:            fmt.Sprintf("⚠️ Это сообщение уже есть в топике «%s».", result.Topic),
				ReplyParameters: &models.ReplyParameters{MessageID: msg.ID},
			})
			h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "duplicate", targetThreadID)
			return
		}
	}

	copied, err := h.forwarder.Duplicate(ctx, msg.Chat.ID, msg.MessageThreadID, targetThreadID, msg.From.Username, fromTopicName, result.Hashtags, msg.ID)
	if err != nil {
		slog.Error("forward failed", "error", err, "msg_id", msg.ID)
		return
	}

	if copied != nil {
		h.forwarder.ReplyWithLink(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID, copied.ID, result.Topic)
	}

	h.markProcessed(ctx, msg.ID, msg.Chat.ID, msg.MessageThreadID, "forwarded", targetThreadID)
}

func (h *Handler) prefilter(msg *models.Message) bool {
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	if text == "" && msg.Document != nil {
		text = msg.Document.FileName
	}

	if text != "" && countWords(text) >= 8 {
		return true
	}

	if text != "" && countWords(text) >= 5 && strings.Contains(text, "http") {
		return true
	}

	if strings.Contains(text, "http") {
		return true
	}

	if (len(msg.Photo) > 0 || msg.Document != nil) && text != "" {
		return true
	}

	return false
}

func (h *Handler) isProcessed(ctx context.Context, messageID int, chatID int64) bool {
	var count int
	err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM processed_messages WHERE message_id = ? AND chat_id = ?`,
		messageID, chatID,
	).Scan(&count)
	return err == nil && count > 0
}

func (h *Handler) markProcessed(ctx context.Context, messageID int, chatID int64, threadID int, action string, targetThreadID int) {
	var threadIDPtr *int
	if threadID != 0 {
		threadIDPtr = &threadID
	}
	h.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO processed_messages (message_id, chat_id, thread_id, action, target_thread_id, processed_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		messageID, chatID, threadIDPtr, action, targetThreadID,
	)
}

func (h *Handler) getTopicName(ctx context.Context, chatID int64, threadID int) string {
	var name string
	h.db.QueryRowContext(ctx, `SELECT name FROM topics WHERE group_id = ? AND tg_thread_id = ?`, chatID, threadID).Scan(&name)
	return name
}

func (h *Handler) lookupThreadID(ctx context.Context, chatID int64, slug string) (int, error) {
	var tgThreadID int
	err := h.db.QueryRowContext(ctx,
		`SELECT tg_thread_id FROM topics WHERE group_id = ? AND slug = ? AND is_active = 1`, chatID, slug,
	).Scan(&tgThreadID)
	return tgThreadID, err
}

func (h *Handler) findOrCreateTopic(ctx context.Context, msg *models.Message, topic string, confidence float64) (int, error) {
	tgThreadID, err := h.lookupThreadID(ctx, msg.Chat.ID, topic)
	if err == nil && tgThreadID != 0 {
		return tgThreadID, nil
	}

	tgThreadID, err = h.fuzzyLookupThreadID(ctx, msg.Chat.ID, topic)
	if err == nil && tgThreadID != 0 {
		slog.Debug("fuzzy topic match", "topic", topic, "thread_id", tgThreadID)
		return tgThreadID, nil
	}

	// Try slug-based match: classifier may latinize differently than our MakeSlug
	tgThreadID, err = h.fuzzySlugMatch(ctx, msg.Chat.ID, topic)
	if err == nil && tgThreadID != 0 {
		slog.Debug("slug-based topic match", "topic", topic, "thread_id", tgThreadID)
		return tgThreadID, nil
	}

	// Auto-create topic for high-confidence important content with no matching topic
	if confidence < 0.90 {
		return 0, fmt.Errorf("no matching topic and confidence too low (%0.2f)", confidence)
	}

	// Convert classifier output to a proper name: "важная-информация" → "Важная информация"
	topicName := strings.ReplaceAll(topic, "-", " ")
	topicName = strings.Title(strings.ToLower(topicName))

	forum, err := h.bot.CreateForumTopic(ctx, &tgbot.CreateForumTopicParams{
		ChatID: msg.Chat.ID,
		Name:   topicName,
	})
	if err != nil {
		slog.Error("auto-create topic failed", "error", err, "name", topicName)
		return 0, fmt.Errorf("create forum topic failed: %w", err)
	}

	slug := util.MakeSlug(topicName)
	h.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO topics (group_id, tg_thread_id, name, slug, is_active) VALUES (?, ?, ?, ?, 1)`,
		msg.Chat.ID, forum.MessageThreadID, topicName, slug,
	)

	slog.Info("auto-created topic for important message",
		"name", topicName, "thread_id", forum.MessageThreadID, "slug", slug,
	)
	return forum.MessageThreadID, nil
}

func mimeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

func resizeImage(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	maxDim := 1024
	if w <= maxDim && h <= maxDim {
		return data, nil
	}
	var newW, newH int
	if w >= h {
		newW = maxDim
		newH = int(float64(h) * float64(maxDim) / float64(w))
	} else {
		newH = maxDim
		newW = int(float64(w) * float64(maxDim) / float64(h))
	}
	if newH < 1 {
		newH = 1
	}
	if newW < 1 {
		newW = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := bounds.Min.Y + (y*h)/newH
		for x := 0; x < newW; x++ {
			srcX := bounds.Min.X + (x*w)/newW
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (h *Handler) classifyMsg(ctx context.Context, chatID int64, text, fileID string) (*classifier.ClassifyResult, error) {
	if fileID == "" {
		return h.classifier.ClassifyMessage(ctx, chatID, text, fileID)
	}

	file, err := h.bot.GetFile(ctx, &tgbot.GetFileParams{FileID: fileID})
	if err != nil {
		return h.classifier.ClassifyMessage(ctx, chatID, text, fileID)
	}

	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", h.token, file.FilePath)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return h.classifier.ClassifyMessage(ctx, chatID, text, fileID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return h.classifier.ClassifyMessage(ctx, chatID, text, fileID)
	}
	defer resp.Body.Close()
	imageData, err := io.ReadAll(resp.Body)
	if err != nil || len(imageData) == 0 {
		return h.classifier.ClassifyMessage(ctx, chatID, text, fileID)
	}

	mime := mimeFromPath(file.FilePath)
	resized, err := resizeImage(imageData)
	if err == nil {
		imageData = resized
		mime = "image/jpeg"
	} else {
		slog.Debug("classify image resize failed, using original", "error", err)
	}

	return h.classifier.ClassifyWithImage(ctx, chatID, text, fileID, imageData, mime)
}

func (h *Handler) fuzzySlugMatch(ctx context.Context, chatID int64, topic string) (int, error) {
	// Tokenize and match against existing slugs by word overlap
	words := strings.FieldsFunc(strings.ToLower(topic), func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	if len(words) == 0 {
		return 0, fmt.Errorf("no words in topic")
	}

	rows, err := h.db.QueryContext(ctx,
		`SELECT slug, tg_thread_id FROM topics WHERE group_id = ? AND is_active = 1`, chatID,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type candidate struct {
		id    int
		slug  string
		score int
	}
	var best candidate
	for rows.Next() {
		var slug string
		var id int
		rows.Scan(&slug, &id)
		score := 0
		slugLower := strings.ToLower(slug)
		for _, w := range words {
			if len(w) >= 3 && strings.Contains(slugLower, w) {
				score++
			}
		}
		if score > best.score {
			best = candidate{id, slug, score}
		}
	}
	if best.score >= 2 || (best.score == 1 && len(words) == 1 && len(words[0]) >= 5) {
		return best.id, nil
	}
	return 0, fmt.Errorf("no slug match for topic: %s", topic)
}

func (h *Handler) fuzzyLookupThreadID(ctx context.Context, chatID int64, topic string) (int, error) {
	// Normalize: classifier may output "важная-информация" while DB has "Важная информация"
	topicNorm := strings.ReplaceAll(strings.ToLower(topic), "-", " ")

	// Try exact name match
	var tgThreadID int
	err := h.db.QueryRowContext(ctx,
		`SELECT tg_thread_id FROM topics WHERE group_id = ? AND LOWER(name) = ? AND is_active = 1 LIMIT 1`,
		chatID, topicNorm,
	).Scan(&tgThreadID)
	if err == nil {
		return tgThreadID, nil
	}

	// Try name contains match (case-insensitive, hyphens normalized)
	rows, err := h.db.QueryContext(ctx,
		`SELECT name, tg_thread_id FROM topics WHERE group_id = ? AND is_active = 1 ORDER BY name`,
		chatID,
	)
	if err == nil {
		defer rows.Close()
		var bestName string
		var bestID int
		for rows.Next() {
			var name string
			var id int
			rows.Scan(&name, &id)
			nameNorm := strings.ReplaceAll(strings.ToLower(name), "-", " ")
			if strings.Contains(nameNorm, topicNorm) || strings.Contains(topicNorm, nameNorm) {
				if bestID == 0 || len(name) < len(bestName) {
					bestID = id
					bestName = name
				}
			}
		}
		if bestID != 0 {
			return bestID, nil
		}
	}

	return 0, fmt.Errorf("no fuzzy match for topic: %s", topic)
}

func (h *Handler) deferMediaGroup(ctx context.Context, b *tgbot.Bot, msg *models.Message, text, fileID string) {
	h.pendingMu.Lock()
	if cancel, ok := h.pendingMediaCancel[msg.MediaGroupID]; ok {
		cancel()
	}
	childCtx, cancel := context.WithCancel(ctx)
	h.pendingMediaCancel[msg.MediaGroupID] = cancel
	h.pendingMu.Unlock()

	go func() {
		select {
		case <-time.After(2 * time.Second):
			h.processDeferredAlbum(childCtx, b, msg, text, fileID)
		case <-childCtx.Done():
			return
		}
	}()
}

func (h *Handler) processDeferredAlbum(ctx context.Context, b *tgbot.Bot, msg *models.Message, text, fileID string) {
	defer func() {
		h.pendingMu.Lock()
		delete(h.pendingMediaCancel, msg.MediaGroupID)
		h.pendingMu.Unlock()
	}()

	result, err := h.classifyMsg(ctx, msg.Chat.ID, text, fileID)
	if err != nil {
		slog.Warn("deferred album classification failed", "error", err)
		return
	}
	if result.Confidence < 0.75 || result.Topic == "" {
		return
	}

	targetThreadID, err := h.findOrCreateTopic(ctx, msg, result.Topic, result.Confidence)
	if err != nil || targetThreadID == 0 {
		slog.Warn("deferred album: target topic not found", "topic", result.Topic, "confidence", result.Confidence)
		return
	}

	if targetThreadID == msg.MessageThreadID {
		return
	}

	rows, err := h.db.QueryContext(ctx,
		`SELECT message_id, file_id, caption FROM media_group_items
		 WHERE media_group_id = ? AND chat_id = ?
		 ORDER BY message_id`,
		msg.MediaGroupID, msg.Chat.ID,
	)
	if err != nil {
		return
	}
	defer rows.Close()
	var groupItems []mediaGroupItem
	for rows.Next() {
		var item mediaGroupItem
		rows.Scan(&item.MessageID, &item.FileID, &item.Caption)
		groupItems = append(groupItems, item)
	}

	if len(groupItems) == 0 {
		return
	}

	slog.Debug("deferred album forward", "count", len(groupItems), "topic", result.Topic)

	if len(groupItems) > 1 {
		var media []models.InputMedia
		hashtagText := ""
		for _, ht := range result.Hashtags {
			ht = strings.ReplaceAll(ht, " ", "-")
			hashtagText += " #" + ht
		}
		for i, item := range groupItems {
			cap := ""
			if i == 0 {
				cap = item.Caption + hashtagText
			}
			media = append(media, &models.InputMediaPhoto{
				Media:                 item.FileID,
				Caption:               cap,
				ShowCaptionAboveMedia: true,
			})
		}
		res, err := b.SendMediaGroup(ctx, &tgbot.SendMediaGroupParams{
			ChatID:          msg.Chat.ID,
			MessageThreadID: targetThreadID,
			Media:           media,
		})
		if err != nil {
			slog.Error("deferred album forward failed", "error", err)
			return
		}
		var newMsgID int
		if len(res) > 0 {
			newMsgID = res[0].ID
		}
		h.forwarder.ReplyWithLink(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID, newMsgID, result.Topic)
	} else {
		copied, err := h.forwarder.Duplicate(ctx, msg.Chat.ID, msg.MessageThreadID, targetThreadID, msg.From.Username, "", result.Hashtags, msg.ID)
		if err != nil {
			slog.Error("deferred single forward failed", "error", err)
			return
		}
		if copied != nil {
			h.forwarder.ReplyWithLink(ctx, msg.Chat.ID, msg.MessageThreadID, msg.ID, copied.ID, result.Topic)
		}
	}

	if len(result.Hashtags) > 0 {
		d := &db.DB{DB: h.db}
		d.AddTags(ctx, msg.Chat.ID, msg.ID, result.Hashtags)
	}

	for _, item := range groupItems {
		h.markProcessed(ctx, item.MessageID, msg.Chat.ID, msg.MessageThreadID, "forwarded", targetThreadID)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func countWords(s string) int {
	if len(s) < 15 {
		return 0
	}
	return len(strings.Fields(s))
}

func textPrefix(s string) string {
	if len(s) > 100 {
		return s[:100]
	}
	return s
}

type mediaGroupItem struct {
	MessageID int
	FileID    string
	Caption   string
}
