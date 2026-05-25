package util

import (
	"regexp"

	"github.com/go-telegram/bot/models"
)

// MessageSender safely extracts user ID and username from a message.
// Returns (userID, username, ok). When ok is false the message has no sender
// (channel post, anonymous admin, service message) and should be skipped.
func MessageSender(msg *models.Message) (userID int64, username string, ok bool) {
	if msg == nil || msg.From == nil {
		return 0, "", false
	}
	return msg.From.ID, msg.From.Username, true
}

// StripMarkdown removes safe markdown formatting patterns from text.
// Only strips paired delimiters: **bold**, `code`, __underline__.
// Does NOT strip single * or _ to avoid breaking normal text.
func StripMarkdown(s string) string {
	s = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile("`(.+?)`").ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(s, "$1")
	return s
}
