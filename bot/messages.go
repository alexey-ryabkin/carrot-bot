package bot

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alexey-ryabkin/carrot-bot/model"
	tele "gopkg.in/telebot.v4"
)

type MessageGroup struct {
	Messages []*tele.Message
}

func messageText(m *tele.Message) string {
	if m == nil {
		return ""
	}
	if m.Text != "" {
		return m.Text
	}
	return m.Caption
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

// labelUser описывает пользователя для логов: id, имя, фамилия, юзернейм.
func labelUser(u *tele.User) string {
	if u == nil {
		return "unknown"
	}
	return fmt.Sprintf("id=%d firstName=%q lastName=%q username=%q",
		u.ID, u.FirstName, u.LastName, u.Username)
}

// labelStoredUser описывает пользователя из базы для логов.
func labelStoredUser(u model.User) string {
	return fmt.Sprintf("id=%d firstName=%q lastName=%q username=%q",
		u.ID, u.FirstName, u.LastName, u.Username)
}

// labelChat описывает чат для логов.
func labelChat(c *tele.Chat) string {
	if c == nil {
		return "unknown"
	}
	return fmt.Sprintf("id=%d type=%s title=%q username=%q",
		c.ID, c.Type, c.Title, c.Username)
}

// labelStoredChat описывает чат из базы для логов.
func labelStoredChat(c model.Chat) string {
	return fmt.Sprintf("id=%d type=%s title=%q username=%q",
		c.ID, c.Type, c.Title, c.Username)
}
