package util

import "fmt"

func GroupDir(chatID int64) string {
	return fmt.Sprintf("data/groups/%d", chatID)
}

func GroupFilePath(chatID int64, filename string) string {
	return fmt.Sprintf("data/groups/%d/%s", chatID, filename)
}
