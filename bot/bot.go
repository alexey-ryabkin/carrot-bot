package bot

import (
	"errors"
	"os"
	"time"

	"github.com/alexey-ryabkin/carrot-bot/storage"
	"github.com/alexey-ryabkin/markov-module"
	tele "gopkg.in/telebot.v4"
)

type Bot struct {
	TeleBot      *tele.Bot
	MyProcessor  *Processor
	Sender       *Sender
	MarkovEngine *markov.Engine
	db           *storage.SQLite
	cfg          Config
}

func New(cfg Config, engine *markov.Engine) (*Bot, error) {
	b, err := tele.NewBot(tele.Settings{
		Token: os.Getenv("TELEGRAM_TOKEN"),
		Poller: &tele.LongPoller{
			Timeout: 10 * time.Second,
		},
	})
	if err != nil {
		return nil, err
	}

	db, err := storage.GetDB(cfg.DatabasePath)
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		TeleBot:      b,
		MyProcessor:  NewProcessor(engine, db),
		Sender:       NewSender(b, engine, db, cfg),
		MarkovEngine: engine,
		db:           db,
		cfg:          cfg,
	}

	b.Handle("/start", start)
	b.Handle(tele.OnText, bot.text)

	return bot, nil
}

func (b *Bot) Start() {
	// tele.Bot.Start() блокирует поллер в горутине — запускаем в фоне,
	// чтобы ниже стартовали фоновые задачи процессора и отправителя.
	go b.TeleBot.Start()
	b.MyProcessor.Start()
	b.Sender.Start()
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
	b.Sender.Close()

	// Здесь:
	// - остановить фоновые задачи
	// - дождаться горутин
	// - закрыть БД
	// - сохранить накопленные данные

	return nil
}
