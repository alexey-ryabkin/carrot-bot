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
	db		 *storage.SQLite

	stop chan struct{}
	done chan struct{}
}

func NewProcessor(engine *markov.Engine, db *storage.SQLite) *Processor {
	return &Processor{
		messages: make([]*tele.Message, 0),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		markov:   engine,
		db:       db,
	}
}

func (p *Processor) Add(message *tele.Message) {
	p.mu.Lock()
	p.messages = append(p.messages, message)
	p.mu.Unlock()
}

func (p *Processor) Start() {
	go func() {
		defer close(p.done)

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				p.process()

			case <-p.stop:
				p.process()
				return
			}
		}
	}()
}

func (p *Processor) process() {
	messages := p.take()

	if len(messages) == 0 {
		return
	}

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

	err := p.markov.LearnMany(markovMessages)
	if err != nil {
		log.Fatal(err)
	}

	err = p.db.SaveMessages(carrotMessages)
	if err != nil {
		log.Fatal(err)
	}

	err = p.db.CleanOldMessages(time.Hour * 24 * 7)
	if err != nil {
		log.Fatal(err)
	}
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
	close(p.stop)
	<-p.done
}
