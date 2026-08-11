package bot

import tele "gopkg.in/telebot.v3"

type MessageGroup struct {
	Messages []*tele.Message
}