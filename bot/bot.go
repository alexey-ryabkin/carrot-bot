package bot

import (
	"errors"
	"os"
	"time"

	"github.com/alexey-ryabkin/markov-module"
	tele "gopkg.in/telebot.v4"
)

type Bot struct {
	TeleBot      *tele.Bot
	MyProcessor  *Processor
	MarkovEngine *markov.Engine
}

func New(engine *markov.Engine) (*Bot, error) {
	b, err := tele.NewBot(tele.Settings{
		Token: os.Getenv("TELEGRAM_TOKEN"),
		Poller: &tele.LongPoller{
			Timeout: 10 * time.Second,
		},
	})
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		TeleBot:      b,
		MyProcessor:  NewProcessor(engine),
		MarkovEngine: engine,
	}

	b.Handle("/start", start)
	b.Handle(tele.OnText, bot.text)

	return bot, nil
}

func (b *Bot) Start() {
	b.TeleBot.Start()
	b.MyProcessor.Start()
}

func start(c tele.Context) error {
	return c.Send("Бот работает")
}

func (b *Bot) text(c tele.Context) error {
	message := c.Message()
	if message == nil {
		return errors.New("No message in OnText")
	}

	b.MyProcessor.Add(message)

	return nil
}

func (b *Bot) Terminate() error {
	b.TeleBot.Stop()
	b.MyProcessor.Close()

	// Здесь:
	// - остановить фоновые задачи
	// - дождаться горутин
	// - закрыть БД
	// - сохранить накопленные данные

	return nil
}
