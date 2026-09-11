package bot

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/alexey-ryabkin/carrot-bot/model"
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

	b.Handle(tele.OnText, bot.handleMessage)
	b.Handle(tele.OnPhoto, bot.handleMessage)
	b.Handle(tele.OnVideo, bot.handleMessage)
	b.Handle(tele.OnDocument, bot.handleMessage)
	b.Handle(tele.OnAudio, bot.handleMessage)
	b.Handle(tele.OnAnimation, bot.handleMessage)
	b.Handle(tele.OnVoice, bot.handleMessage)

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

func (b *Bot) handleMessage(c tele.Context) error {
	message := c.Message()
	if message == nil {
		log.Printf("handleMessage вызван без сообщения")
		return errors.New("no message in handleMessage")
	}

	text := messageText(message)
	if text == "" {
		return nil
	}

	// Лог полученного сообщения
	if message.Sender != nil {
		if err := b.db.UpsertUser(model.User{
			ID:        message.Sender.ID,
			FirstName: message.Sender.FirstName,
			LastName:  message.Sender.LastName,
			Username:  message.Sender.Username,
		}); err != nil {
			log.Printf("сохранение пользователя в базу: %v", err)
		}
	}

	if message.Chat != nil {
		if err := b.db.UpsertChat(model.Chat{
			ID:       message.Chat.ID,
			Type:     string(message.Chat.Type),
			Title:    message.Chat.Title,
			Username: message.Chat.Username,
		}); err != nil {
			log.Printf("сохранение чата в базу: %v", err)
		}

		log.Printf("получено сообщение: %s msgid=%d from=%s text=%q",
			labelChat(message.Chat), message.ID,
			labelUser(message.Sender), logText(text))
	} else {
		log.Printf("получено сообщение без чата: msgid=%d from=%s text=%q",
			message.ID, labelUser(message.Sender), logText(text))
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
