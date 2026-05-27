package main

import (
	"testing"
	"unicode/utf16"

	"omsu_bot/internal/util"

	"github.com/go-telegram/bot/models"
)

func TestIsBotMention(t *testing.T) {
	// Setup global botUsername for test
	oldBotUsername := botUsername
	botUsername = "my_test_bot"

	defer func() {
		botUsername = oldBotUsername
	}()

	tests := []struct {
		name    string
		message *models.Message
		want    bool
	}{
		{
			name: "Correct mention in text",
			message: &models.Message{
				Text: "@my_test_bot hello there",
				Entities: []models.MessageEntity{
					{
						Type:   models.MessageEntityTypeMention,
						Offset: 0,
						Length: 12, // "@my_test_bot" has length 12
					},
				},
			},
			want: true,
		},
		{
			name: "Correct mention in caption",
			message: &models.Message{
				Caption: "@my_test_bot check this out",
				CaptionEntities: []models.MessageEntity{
					{
						Type:   models.MessageEntityTypeMention,
						Offset: 0,
						Length: 12,
					},
				},
			},
			want: true,
		},
		{
			name: "Case insensitive matching",
			message: &models.Message{
				Text: "Hello @MY_TEST_BOT!",
				Entities: []models.MessageEntity{
					{
						Type:   models.MessageEntityTypeMention,
						Offset: 6,
						Length: 12,
					},
				},
			},
			want: true,
		},
		{
			name: "Mention of another bot/user",
			message: &models.Message{
				Text: "@someone_else check this",
				Entities: []models.MessageEntity{
					{
						Type:   models.MessageEntityTypeMention,
						Offset: 0,
						Length: 13,
					},
				},
			},
			want: false,
		},
		{
			name: "No entities",
			message: &models.Message{
				Text: "Hello world without mentions",
			},
			want: false,
		},
		{
			name: "Entities exist but no mentions",
			message: &models.Message{
				Text: "Visit google.com",
				Entities: []models.MessageEntity{
					{
						Type:   models.MessageEntityTypeURL,
						Offset: 6,
						Length: 10,
					},
				},
			},
			want: false,
		},
		{
			name: "UTF-16 offset handling with emojis",
			message: &models.Message{
				// Emoji "👋" takes 2 UTF-16 code units (surrogate pair) in some encodings, but let's test a simple surrogate pair:
				// "👋 @my_test_bot"
				// "👋" is U+1F44B, which in UTF-16 is 2 code units: 0xD83D 0xDC4B.
				// So length of "👋" in UTF-16 code units is 2. The space is 1. The offset of mention is 3.
				Text: string(utf16.Decode([]uint16{0xD83D, 0xDC4B})) + " @my_test_bot",
				Entities: []models.MessageEntity{
					{
						Type:   models.MessageEntityTypeMention,
						Offset: 3,
						Length: 12,
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBotMention(tt.message)
			if got != tt.want {
				t.Errorf("isBotMention() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMakeSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Привет Мир", "privet_mir"},
		{"Тест-Тест...123", "test_test123"},
		{"slug.with.dots", "slugwithdots"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := util.MakeSlug(tt.input)
			if got != tt.want {
				t.Errorf("makeSlug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
