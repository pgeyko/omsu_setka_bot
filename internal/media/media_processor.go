package media

import (
	"bytes"
	"context"
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
	"time"

	"omsu_bot/internal/llm"

	tgbot "github.com/go-telegram/bot"
)

const maxPhotoDimension = 512

// MediaProcessor downloads and processes Telegram media files (photos, voice).
// For photos it resizes to ≤512px before sending to the LLM vision API.
// When no multimodal provider is available, ProcessPhoto returns ("", nil) so
// the caller can treat the message as text-only without error.
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

// ProcessPhoto downloads a Telegram photo, resizes it to ≤512px, and runs OCR
// via the LLM vision API. Returns ("", nil) when no multimodal provider is
// active — the caller should continue without vision rather than treating this
// as an error.
func (mp *MediaProcessor) ProcessPhoto(ctx context.Context, fileID string) (string, error) {
	if !mp.llmClient.HasMultimodalProvider() {
		slog.Debug("no multimodal provider available, skipping photo OCR", "file_id", fileID)
		return "", nil
	}

	data, mimeType, err := mp.downloadFile(ctx, fileID)
	if err != nil {
		return "", err
	}

	// Resize if needed (only for images, not docs).
	if strings.HasPrefix(mimeType, "image/") {
		resized, resizedMime, err := resizeIfNeeded(data, mimeType)
		if err != nil {
			// Non-fatal: log and fall back to original.
			slog.Warn("photo resize failed, using original", "error", err)
		} else {
			data = resized
			mimeType = resizedMime
		}
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

// ProcessVoice downloads a Telegram voice message and transcribes it through
// the vision chain (gemma-4-31b-it → gemini-3.1-flash-lite).
// Returns ("", nil) when no multimodal provider is active.
func (mp *MediaProcessor) ProcessVoice(ctx context.Context, fileID string) (string, error) {
	if !mp.llmClient.HasMultimodalProvider() {
		slog.Debug("no multimodal provider available, skipping voice STT", "file_id", fileID)
		return "", nil
	}

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

// resizeIfNeeded decodes the image and resizes it proportionally so that the
// longest side is ≤ maxPhotoDimension. Always re-encodes as JPEG to keep the
// payload small. If the image is already small enough, the original bytes are
// returned.
func resizeIfNeeded(data []byte, _ string) ([]byte, string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxPhotoDimension && h <= maxPhotoDimension {
		// Already within limits — re-encode as JPEG anyway for consistency.
		return encodeJPEG(img)
	}

	// Compute new dimensions keeping aspect ratio.
	var newW, newH int
	if w >= h {
		newW = maxPhotoDimension
		newH = int(float64(h) * float64(maxPhotoDimension) / float64(w))
	} else {
		newH = maxPhotoDimension
		newW = int(float64(w) * float64(maxPhotoDimension) / float64(h))
	}
	if newH < 1 {
		newH = 1
	}
	if newW < 1 {
		newW = 1
	}

	resized := resampleNearest(img, newW, newH)
	return encodeJPEG(resized)
}

// resampleNearest is a simple nearest-neighbour resample.
// We use stdlib only (no external dependency) — quality is sufficient for OCR.
func resampleNearest(src image.Image, newW, newH int) image.Image {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := srcBounds.Min.Y + (y*srcH)/newH
		for x := 0; x < newW; x++ {
			srcX := srcBounds.Min.X + (x*srcW)/newW
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func encodeJPEG(img image.Image) ([]byte, string, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "", fmt.Errorf("jpeg encode: %w", err)
	}
	return buf.Bytes(), "image/jpeg", nil
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

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
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
	case ".gif":
		return "image/gif"
	case ".ogg", ".oga":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}
