package bot

import (
	"log"
	"sync"
	"time"

	"github.com/alexey-ryabkin/markov-module"
	"github.com/alexey-ryabkin/markov-module/model"
	tele "gopkg.in/telebot.v4"
)

type Processor struct {
	mu       sync.Mutex
	messages []*tele.Message
	markov   *markov.Engine

	stop chan struct{}
	done chan struct{}
}

func NewProcessor(engine *markov.Engine) *Processor {
	return &Processor{
		messages: make([]*tele.Message, 0),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		markov:   engine,
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

	markov_messages := make([]model.Message, 0, len(messages))

	// Здесь обрабатывается вся группа.
	for _, message := range messages {
		if message.Sender == nil {
			continue
		}
		markov_messages = append(markov_messages, model.Message{
			ChatId:   message.Chat.ID,
			UserId:   message.Sender.ID,
			UnixTime: message.Unixtime,
			Text:     message.Text,
		})
	}

	err := p.markov.LearnMany(markov_messages)
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
