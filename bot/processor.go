package bot

import (
	"log"
	"sync"
	"time"

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
	cfg      Config

	stop chan struct{}
	done chan struct{}
}

func NewProcessor(engine *markov.Engine, db *storage.SQLite, cfg Config) *Processor {
	return &Processor{
		messages: make([]*tele.Message, 0),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		markov:   engine,
		db:       db,
		cfg:      cfg,
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

	if message.Chat != nil {
		log.Printf("сообщение поставлено в очередь: chat=%d msgid=%d queue=%d", message.Chat.ID, message.ID, queued)
	} else {
		log.Printf("сообщение поставлено в очередь без чата: msgid=%d queue=%d", message.ID, queued)
	}
}

func (p *Processor) Start() {
	go func() {
		defer close(p.done)

		log.Printf("процессор запущен, сброс каждые %.1f с", p.cfg.QueueReadInterval.Seconds())

		ticker := time.NewTicker(p.cfg.QueueReadInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
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

	// Генерация

}

func (p *Processor) learn(messages []*tele.Message) {
	markovMessages := make([]markovModel.Message, 0, len(messages))
	carrotMessages := make([]model.Message, 0, len(messages))

	for _, message := range messages {
		if message.Sender == nil {
			continue
		}
		markovMessages = append(markovMessages, markovModel.Message{
			ChatId:   message.Chat.ID,
			UserId:   message.Sender.ID,
			UnixTime: message.Unixtime,
			Text:     message.Text,
		})
		carrotMessages = append(carrotMessages, model.Message{
			ChatId:   message.Chat.ID,
			UserId:   message.Sender.ID,
			UnixTime: message.Unixtime,
		})
	}

	log.Printf("обучение: batch=%d usable=%d пропущено=%d (нет отправителя)",
		len(messages), len(carrotMessages), len(messages)-len(carrotMessages))

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

	err = p.db.CleanOldMessages(time.Hour * 24 * 7)
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
