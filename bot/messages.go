package bot

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tele "gopkg.in/telebot.v4"
)

type MessageGroup struct {
	Messages []*tele.Message
}

// logText подготавливает текст для записи в лог: схлопывает переводы строк
// и усекает слишком длинные сообщения.
func logText(text string) string {
	const maxRunes = 300

	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > maxRunes {
		runes := []rune(text)
		text = string(runes[:maxRunes]) + "…"
	}
	return text
}

// labelUser описывает пользователя для логов в виде "id (имя)".
func labelUser(u *tele.User) string {
	if u == nil {
		return "unknown"
	}

	name := ""
	switch {
	case u.Username != "":
		name = "@" + u.Username
	case u.FirstName != "" || u.LastName != "":
		name = strings.TrimSpace(u.FirstName + " " + u.LastName)
	}

	if name == "" {
		return fmt.Sprintf("%d", u.ID)
	}
	return fmt.Sprintf("%d (%s)", u.ID, name)
}
