package util

import (
	"fmt"
	"strings"
)

var cyrToLat = strings.NewReplacer(
	"а", "a", "б", "b", "в", "v", "г", "g", "д", "d", "е", "e", "ё", "e",
	"ж", "zh", "з", "z", "и", "i", "й", "y", "к", "k", "л", "l", "м", "m",
	"н", "n", "о", "o", "п", "p", "р", "r", "с", "s", "т", "t", "у", "u",
	"ф", "f", "х", "kh", "ц", "ts", "ч", "ch", "ш", "sh", "щ", "shch",
	"ы", "y", "э", "e", "ю", "yu", "я", "ya",
)

func MakeSlug(name string) string {
	slug := strings.ToLower(name)
	slug = cyrToLat.Replace(slug)
	slug = strings.ReplaceAll(slug, " ", "_")
	slug = strings.ReplaceAll(slug, ".", "")
	slug = strings.ReplaceAll(slug, "-", "_")
	slug = strings.ReplaceAll(slug, "'", "")
	slug = strings.ReplaceAll(slug, "`", "")
	slug = strings.ReplaceAll(slug, "\"", "")
	return slug
}

func ChatLink(chatID int64, msgID int) string {
	if chatID < 0 {
		s := fmt.Sprintf("%d", -chatID)
		if strings.HasPrefix(s, "100") && len(s) > 3 {
			s = s[3:]
		}
		return fmt.Sprintf("https://t.me/c/%s/%d", s, msgID)
	}
	return fmt.Sprintf("https://t.me/c/%d/%d", chatID, msgID)
}
