package bot

import tele "gopkg.in/telebot.v4"

type MessageGroup struct {
	Messages []*tele.Message
}