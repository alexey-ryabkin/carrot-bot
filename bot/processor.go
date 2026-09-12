package bot

import (
	"log"
	"sync"
	"time"

	"github.com/alexey-ryabkin/carrot-bot/config"
	"github.com/alexey-ryabkin/carrot-bot/model"
	"github.com/alexey-ryabkin/carrot-bot/storage"
	"github.com/alexey-ryabkin/markov-module"
	markovModel "github.com/alexey-ryabkin/markov-module/model"
	tele "gopkg.in/telebot.v4"
)

type Processor struct {
	mu       sync.Mutex
	messages []*tele.Message
	markov   *markov.Engine
	db       *storage.SQLite
	cfg      *config.Service

	stop    chan struct{}
	done    chan struct{}
	trigger chan struct{}
}

func NewProcessor(engine *markov.Engine, db *storage.SQLite, cfg *config.Service) *Processor {
	return &Processor{
		messages: make([]*tele.Message, 0),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		trigger:  make(chan struct{}, 1),
		markov:   engine,
		db:       db,
		cfg:      cfg,
	}
}

func (p *Processor) Trigger() {
	select {
	case p.trigger <- struct{}{}:
		log.Printf("процессор: запрошен ручной запуск")
	default:
		log.Printf("процессор: ручной запуск уже запланирован")
	}
}

func (p *Processor) Add(message *tele.Message) {
	if message == nil {
		log.Printf("Add(nil) проигнорировано")
		return
	}

	p.mu.Lock()
	p.messages = append(p.messages, message)
	queued := len(p.messages)
	p.mu.Unlock()

	text := messageText(message)
	log.Printf("сообщение поставлено в очередь: %s msgid=%d queue=%d from=%s text=%q",
		labelChat(message.Chat), message.ID, queued, labelUser(message.Sender), logText(text))
}

func (p *Processor) Start() {
	go func() {
		defer close(p.done)

		log.Printf("процессор запущен, сброс каждые %v", p.cfg.QueueReadInterval())

		ticker := time.NewTicker(p.cfg.QueueReadInterval())
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				p.process()

			case <-p.trigger:
				log.Printf("процессор: ручной запуск")
				p.process()

			case <-p.stop:
				log.Printf("получен сигнал остановки, сбрасываю оставшиеся сообщения")
				p.process()
				log.Printf("процессор остановлен")
				return
			}
		}
	}()
}

func (p *Processor) process() {
	messages := p.take()

	if len(messages) == 0 {
		log.Printf("тик: очередь пуста")
		return
	}
	log.Printf("тик: обработка %d сообщений из очереди", len(messages))

	// Обучение
	p.learn(messages)

}

func (p *Processor) learn(messages []*tele.Message) {
	markovMessages := make([]markovModel.Message, 0, len(messages))
	carrotMessages := make([]model.Message, 0, len(messages))

	for _, message := range messages {
		text := messageText(message)

		if message.Sender == nil || message.Chat == nil {
			continue
		}

		chat := model.Chat{
			ID:       message.Chat.ID,
			Type:     string(message.Chat.Type),
			Title:    message.Chat.Title,
			Username: message.Chat.Username,
		}
		user := model.User{
			ID:        message.Sender.ID,
			FirstName: message.Sender.FirstName,
			LastName:  message.Sender.LastName,
			Username:  message.Sender.Username,
		}

		if err := p.db.UpsertChat(chat); err != nil {
			log.Printf("сохранение чата в базу: %v", err)
		}

		if err := p.db.UpsertUser(user, chat); err != nil {
			log.Printf("сохранение пользователя в базу: %v", err)
		}

		carrotMessages = append(carrotMessages, model.Message{
			ChatId:   message.Chat.ID,
			UserId:   message.Sender.ID,
			UnixTime: message.Unixtime,
		})

		if text == "" {
			continue
		}
		markovMessages = append(markovMessages, markovModel.Message{
			ChatId:   message.Chat.ID,
			UserId:   message.Sender.ID,
			UnixTime: message.Unixtime,
			Text:     text,
		})
	}

	log.Printf("обучение: batch=%d activity=%d markov=%d пропущено=%d (нет отправителя/чата)",
		len(messages), len(carrotMessages), len(markovMessages), len(messages)-len(carrotMessages))

	err := p.markov.LearnMany(markovMessages)
	if err != nil {
		log.Fatalf("ошибка обучения (LearnMany, %d сообщений): %v", len(markovMessages), err)
	}
	log.Printf("марковская цепь обучена на %d сообщениях", len(markovMessages))

	err = p.db.SaveMessages(carrotMessages)
	if err != nil {
		log.Fatalf("ошибка сохранения (SaveMessages, %d сообщений): %v", len(carrotMessages), err)
	}
	log.Printf("в кэш активности сохранено %d сообщений", len(carrotMessages))

	err = p.db.CleanOldMessages(p.cfg.GlobalWindow())
	if err != nil {
		log.Fatalf("ошибка очистки старых сообщений (CleanOldMessages): %v", err)
	}
	log.Printf("кэш активности очищен от старых сообщений")
}

func (p *Processor) take() []*tele.Message {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.messages) == 0 {
		return nil
	}

	messages := p.messages
	p.messages = make([]*tele.Message, 0)

	return messages
}

func (p *Processor) Close() {
	log.Printf("закрытие процессора")
	close(p.stop)
	<-p.done
}
