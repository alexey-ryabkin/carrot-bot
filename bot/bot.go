package bot

import (
	"errors"
	"log"
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
	log.Printf("создание Telegram-клиента, TELEGRAM_TOKEN задан: %t", os.Getenv("TELEGRAM_TOKEN") != "")

	b, err := tele.NewBot(tele.Settings{
		Token: os.Getenv("TELEGRAM_TOKEN"),
		Poller: &tele.LongPoller{
			Timeout: 10 * time.Second,
		},
	})
	if err != nil {
		return nil, err
	}
	log.Printf("Telegram-клиент создан, id=%d username=%s", b.Me.ID, b.Me.Username)

	db, err := storage.GetDB(cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	log.Printf("база активности готова: %s", cfg.DatabasePath)

	bot := &Bot{
		TeleBot:      b,
		MyProcessor:  NewProcessor(engine, db, cfg),
		Sender:       NewSender(b, engine, db, cfg),
		MarkovEngine: engine,
		db:           db,
		cfg:          cfg,
	}

	b.Handle("/start", start)
	b.Handle(tele.OnText, bot.text)

	log.Printf("бот инициализирован")

	return bot, nil
}

func (b *Bot) Start() {
	b.MyProcessor.Start()
	b.Sender.Start()

	log.Printf("бот запущен (поллер, процессор, отправитель)")

	b.TeleBot.Start()
	log.Printf("поллер Telegram остановлен")
}

func start(c tele.Context) error {
	if u := c.Sender(); u != nil {
		log.Printf("команда /start от пользователя %s", labelUser(u))
	} else {
		log.Printf("команда /start от неизвестного пользователя")
	}
	return c.Send("Бот работает")
}

func (b *Bot) text(c tele.Context) error {
	message := c.Message()
	if message == nil {
		log.Printf("OnText вызван без сообщения")
		return errors.New("No message in OnText")
	}

	if message.Chat != nil {
		log.Printf("получено сообщение: chat=%d type=%s title=%q msgid=%d from=%s text=%q",
			message.Chat.ID, message.Chat.Type, message.Chat.Title, message.ID,
			labelUser(message.Sender), logText(message.Text))
	} else {
		log.Printf("получено сообщение без чата: msgid=%d from=%s text=%q",
			message.ID, labelUser(message.Sender), logText(message.Text))
	}

	b.MyProcessor.Add(message)

	return nil
}

func (b *Bot) Terminate() error {
	log.Printf("завершение работы")
	b.TeleBot.Stop()
	b.MyProcessor.Close()
	b.Sender.Close()

	// Здесь:
	// - остановить фоновые задачи
	// - дождаться горутин
	// - закрыть БД
	// - сохранить накопленные данные

	log.Printf("работа завершена")
	return nil
}
