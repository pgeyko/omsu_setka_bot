package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"omsu_bot/internal/llm"
	"omsu_bot/internal/util"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type ScheduleIntent struct {
	Intent       string  `json:"intent"`
	Date         string  `json:"date"`
	RelativeDate string  `json:"relative_date"`
	Subgroup     int     `json:"subgroup"`
	Pair         int     `json:"pair"`
	Subject      string  `json:"subject"`
	Field        string  `json:"field"`
	Confidence   float64 `json:"confidence"`
}

type ScheduleQueryHandler struct {
	llmClient    *llm.Client
	prompts      *llm.PromptRegistry
	setkaBaseURL string
	setkaPublic  string
	omsuGroupID  int
	cmd          *CommandRegistry
}

func NewScheduleQueryHandler(llmClient *llm.Client, prompts *llm.PromptRegistry, setkaBaseURL, setkaPublic string, omsuGroupID int, cmd *CommandRegistry) *ScheduleQueryHandler {
	return &ScheduleQueryHandler{
		llmClient:    llmClient,
		prompts:      prompts,
		setkaBaseURL: setkaBaseURL,
		setkaPublic:  setkaPublic,
		omsuGroupID:  omsuGroupID,
		cmd:          cmd,
	}
}

func (h *ScheduleQueryHandler) Handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	msg := update.Message
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	if text == "" {
		return
	}

	today := time.Now().Format("2006-01-02")
	prompt := h.prompts.Get("schedule_query")
	prompt = strings.ReplaceAll(prompt, "{today}", today)
	prompt = strings.ReplaceAll(prompt, "{text}", text)

	resp, err := h.llmClient.Call(ctx, "schedule_query", "", prompt, false)
	if err != nil {
		slog.Error("schedule query LLM failed", "error", err)
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID, "Не удалось обработать запрос. Попробуй ещё раз.")
		return
	}

	var intent ScheduleIntent
	if err := json.Unmarshal([]byte(llm.ExtractJSON(resp.Content)), &intent); err != nil || intent.Intent != "schedule_query" || intent.Confidence < 0.7 {
		slog.Error("failed to parse schedule intent", "error", err)
		return
	}

	date := util.ResolveDate(intent.Date, intent.RelativeDate)
	if date == "" {
		date = today
	}

	url := fmt.Sprintf("%s/api/v1/schedule/group/%d/day?date=%s", h.setkaBaseURL, h.omsuGroupID, date)
	httpClient := &http.Client{Timeout: 15 * time.Second}
	httpResp, err := httpClient.Get(url)
	if err != nil {
		slog.Error("failed to fetch schedule from setka", "error", err)
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID, "Не удалось получить расписание. Сервис временно недоступен.")
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		slog.Error("failed to read setka response", "error", err)
		return
	}

	var wrapper struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil || !wrapper.Success {
		slog.Error("invalid setka response", "error", err)
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID, "Ошибка при получении расписания.")
		return
	}

	scheduleJSON, _ := json.Marshal(wrapper.Data)

	answerPrompt := h.prompts.Get("schedule_answer")
	answerPrompt = strings.ReplaceAll(answerPrompt, "{query}", text)
	answerPrompt = strings.ReplaceAll(answerPrompt, "{date}", date)
	answerPrompt = strings.ReplaceAll(answerPrompt, "{schedule_json}", string(scheduleJSON))

	answer, err := h.llmClient.Call(ctx, "schedule_answer", "", answerPrompt, false)
	if err != nil {
		slog.Error("schedule answer LLM failed", "error", err)
		h.reply(ctx, b, msg.Chat.ID, msg.MessageThreadID, msg.ID, "Не удалось сформировать ответ.")
		return
	}

	answerText := answer.Content
	if h.setkaPublic != "" {
		setkaLink := h.setkaPublic
		if !strings.Contains(setkaLink, "/schedule/group") {
			setkaLink = strings.TrimRight(setkaLink, "/") + fmt.Sprintf("/schedule/group/%d?date=%s", h.omsuGroupID, date)
		}
		answerText += fmt.Sprintf("\n\n📅 <a href=\"%s\">Открыть расписание</a>", setkaLink)
	}

	b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          msg.Chat.ID,
		MessageThreadID: msg.MessageThreadID,
		Text:            answerText,
		ParseMode:       models.ParseModeHTML,
	})
}

func (h *ScheduleQueryHandler) reply(ctx context.Context, b *tgbot.Bot, chatID int64, threadID int, replyToID int, text string) {
	b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		Text:            text,
		ReplyParameters: &models.ReplyParameters{MessageID: replyToID},
	})
}


