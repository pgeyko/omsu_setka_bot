package util

import "strings"

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
