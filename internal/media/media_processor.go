package media

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"omsu_bot/internal/llm"

	tgbot "github.com/go-telegram/bot"
)

type MediaProcessor struct {
	botClient *tgbot.Bot
	token     string
	llmClient *llm.Client
}

func NewMediaProcessor(botClient *tgbot.Bot, token string, llmClient *llm.Client) *MediaProcessor {
	return &MediaProcessor{
		botClient: botClient,
		token:     token,
		llmClient: llmClient,
	}
}

func (mp *MediaProcessor) ProcessPhoto(ctx context.Context, fileID string) (string, error) {
	data, mimeType, err := mp.downloadFile(ctx, fileID)
	if err != nil {
		return "", err
	}

	history := []llm.AgentMessage{
		{
			Role:    "user",
			Content: "Распознай и запиши весь текст с этого изображения. Ответь только распознанным текстом без форматирования и без лишних пояснений. Если текста нет, напиши '[Изображение без распознаваемого текста]'.",
			MediaParts: []llm.MediaPart{
				{
					MimeType: mimeType,
					Data:     data,
				},
			},
		},
	}

	resp, err := mp.llmClient.CallGroupHistory(ctx, 0, "ocr", "", history, nil, true)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func (mp *MediaProcessor) ProcessVoice(ctx context.Context, fileID string) (string, error) {
	data, mimeType, err := mp.downloadFile(ctx, fileID)
	if err != nil {
		return "", err
	}

	if mimeType == "application/octet-stream" || strings.HasSuffix(mimeType, "octet-stream") {
		mimeType = "audio/ogg"
	}

	history := []llm.AgentMessage{
		{
			Role:    "user",
			Content: "Сделай дословную текстовую расшифровку этой аудиозаписи. Напиши только расшифрованный текст без форматирования, комментариев и метаданных. Если голоса нет или расшифровка невозможна, напиши '[Не удалось расшифровать аудио]'.",
			MediaParts: []llm.MediaPart{
				{
					MimeType: mimeType,
					Data:     data,
				},
			},
		},
	}

	resp, err := mp.llmClient.CallGroupHistory(ctx, 0, "stt", "", history, nil, true)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func (mp *MediaProcessor) downloadFile(ctx context.Context, fileID string) ([]byte, string, error) {
	file, err := mp.botClient.GetFile(ctx, &tgbot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file path from Telegram: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", mp.token, file.FilePath)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("telegram returned non-200 status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	mimeType := getMimeTypeByPath(file.FilePath)
	return data, mimeType, nil
}

func getMimeTypeByPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".ogg", ".oga":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}
