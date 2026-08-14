package bot

import (
	"sync"
	"time"

	tele "gopkg.in/telebot.v4"
)

type Processor struct {
	mu       sync.Mutex
	messages []*tele.Message

	stop chan struct{}
	done chan struct{}
}

func NewProcessor() *Processor {
	return &Processor{
		messages: make([]*tele.Message, 0),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
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

	// Здесь обрабатывается вся группа.
	for _, message := range messages {
		// ...
		_ = message
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