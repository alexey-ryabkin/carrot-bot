package bot

import (
	"os"
	"time"

	tele "gopkg.in/telebot.v3"
)

type Bot struct {
	*tele.Bot
}

func New() (*Bot, error) {
	b, err := tele.NewBot(tele.Settings{
		Token: os.Getenv("TELEGRAM_TOKEN"),
		Poller: &tele.LongPoller{
			Timeout: 10 * time.Second,
		},
	})
	if err != nil {
		return nil, err
	}

	b.Handle("/start", start)
	b.Handle(tele.OnText, text)

	return &Bot{Bot: b}, nil
}

func start(c tele.Context) error {
	return c.Send("Бот работает")
}

func text(c tele.Context) error {
	return c.Send(c.Text())
}

func (b *Bot) Terminate() error {
	b.Stop()

	// Здесь:
	// - остановить фоновые задачи
	// - дождаться горутин
	// - закрыть БД
	// - сохранить накопленные данные

	return nil
}