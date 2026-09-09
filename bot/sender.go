package bot

import (
	"log"
	"math/rand"
	"time"

	"github.com/alexey-ryabkin/carrot-bot/model"
	"github.com/alexey-ryabkin/carrot-bot/probability"
	"github.com/alexey-ryabkin/carrot-bot/storage"
	"github.com/alexey-ryabkin/markov-module"
	tele "gopkg.in/telebot.v4"
)

// ---- Sender: регулярная проверка, нужно ли отправить сообщение ----

// Sender регулярно проверяет каждый чат: не пора ли отправить сгенерированное сообщение.
type Sender struct {
	tele   *tele.Bot
	markov *markov.Engine
	db     *storage.SQLite
	cfg    Config

	// ID бота, которым записываются собственные сообщения в кэш активности.
	botID int64

	stop chan struct{}
	done chan struct{}
}

func NewSender(tele *tele.Bot, engine *markov.Engine, db *storage.SQLite, cfg Config) *Sender {
	var botID int64
	if tele.Me != nil {
		botID = tele.Me.ID
	}

	log.Printf("отправитель создан: checkInterval=%v minUserWeight=%v globalWindow=%v botID=%d params=%+v",
		cfg.SendCheckInterval, cfg.MinUserWeight, cfg.GlobalWindow, botID, cfg.ProbabilityParams)

	return &Sender{
		tele:   tele,
		markov: engine,
		db:     db,
		cfg:    cfg,
		botID:  botID,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start запускает фоновую проверку по таймеру.
func (s *Sender) Start() {
	go func() {
		defer close(s.done)

		log.Printf("отправитель запущен, проверка каждые %v", s.cfg.SendCheckInterval)

		ticker := time.NewTicker(s.cfg.SendCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.check()
			case <-s.stop:
				log.Printf("отправитель остановлен")
				return
			}
		}
	}()
}

// Close останавливает фоновую проверку и дожидается выхода из горутины.
func (s *Sender) Close() {
	log.Printf("закрытие отправителя")
	close(s.stop)
	<-s.done
}

// check проходит по всем известным чатам и независимо решает про каждый.
func (s *Sender) check() {
	chats, err := s.db.GetChats()
	if err != nil {
		log.Printf("ошибка получения списка чатов: %v", err)
		return
	}
	log.Printf("проверка: %d известных чатов", len(chats))

	sent := 0
	for _, chatID := range chats {
		if s.maybeSend(chatID) {
			sent++
		}
	}
	log.Printf("проверка завершена: chats=%d sent=%d", len(chats), sent)
}

func (s *Sender) maybeSend(chatID int64) bool {
	now := time.Now()

	ok, err := s.shouldSend(chatID, now)
	if err != nil {
		log.Printf("shouldSend, чат %d: %v", chatID, err)
		return false
	}
	if !ok {
		return false
	}

	userID, err := s.pickUser(chatID, now)
	if err != nil {
		log.Printf("pickUser, чат %d: %v", chatID, err)
		return false
	}
	if userID == 0 {
		log.Printf("pickUser, чат %d: пользователь не выбран", chatID)
		return false
	}

	text, err := s.markov.Generate(chatID, userID)
	if err != nil {
		log.Printf("генерация, чат %d, пользователь %d: %v", chatID, userID, err)
		return false
	}
	if text == "" {
		log.Printf("генерация, чат %d, пользователь %d: пустой текст, пропуск", chatID, userID)
		return false
	}
	log.Printf("генерация, чат %d: текст (%d символов) для пользователя %d: %q",
		chatID, len(text), userID, logText(text))

	if err := s.send(chatID, text); err != nil {
		log.Printf("отправка в чат %d: %v", chatID, err)
		return false
	}

	if err := s.db.SaveMessages([]model.Message{{
		ChatId:   chatID,
		UserId:   s.botID,
		UnixTime: now.Unix(),
	}}); err != nil {
		log.Printf("сохранение сообщения бота, чат %d: %v", chatID, err)
	} else {
		log.Printf("чат %d: сообщение бота сохранено в кэш активности", chatID)
	}
	return true
}

func (s *Sender) send(chatID int64, text string) error {
	msg, err := s.tele.Send(&tele.Chat{ID: chatID}, text)
	if err != nil {
		return err
	}
	log.Printf("чат %d: сообщение отправлено, msgid=%d", chatID, msg.ID)
	return nil
}

// shouldSend решает, пора ли отправить сообщение в чат.
func (s *Sender) shouldSend(chatID int64, now time.Time) (bool, error) {
	localWindow := s.LocalWindow(s.cfg.SendCheckInterval)
	localCount, err := s.db.CountMessagesPeople(chatID, s.botID, now.Add(-localWindow).Unix())
	if err != nil {
		return false, err
	}

	weekCount, err := s.db.CountMessagesPeople(chatID, s.botID, now.Add(-s.cfg.GlobalWindow).Unix())
	if err != nil {
		return false, err
	}

	lastBotTime, err := s.db.GetLastActivityUser(chatID, s.botID)
	if err != nil {
		return false, err
	}
	sinceBotCount, err := s.db.CountMessagesPeople(chatID, s.botID, lastBotTime)
	if err != nil {
		return false, err
	}

	send := probability.ShouldSend(
		localCount,
		weekCount,
		sinceBotCount,
		localWindow,
		s.cfg.GlobalWindow,
		s.cfg.SendCheckInterval,
		s.cfg.ProbabilityParams,
	)
	log.Printf("shouldSend, чат %d: localCount=%d weekCount=%d sinceBotCount=%d localWindow=%v → %t",
		chatID, localCount, weekCount, sinceBotCount, localWindow, send)

	return send, nil
}

func (s *Sender) pickUser(chatID int64, now time.Time) (int64, error) {
	users, err := s.db.GetUsers(chatID)
	if err != nil {
		return 0, err
	}

	windowStart := now.Add(-s.cfg.GlobalWindow).Unix()
	ids := make([]int64, 0, len(users))
	weights := make([]float64, 0, len(users))
	var total float64

	for _, userID := range users {
		count, err := s.db.CountMessagesUser(chatID, userID, windowStart)
		if err != nil {
			return 0, err
		}
		w := float64(count) + s.cfg.MinUserWeight
		ids = append(ids, userID)
		weights = append(weights, w)
		total += w
	}

	if len(ids) == 0 {
		log.Printf("pickUser, чат %d: нет кандидатов (пользователей в чате: %d)", chatID, len(users))
		return 0, nil
	}

	idx := weightedIndex(weights, total)

	chosen := ids[idx]
	storedUser, err := s.db.GetUser(chosen)
	if err != nil {
		log.Printf("pickUser, чат %d: candidates=%d weightsTotal=%.1f chosen=id=%d (данные пользователя не найдены: %v)",
			chatID, len(ids), total, chosen, err)
	} else {
		log.Printf("pickUser, чат %d: candidates=%d weightsTotal=%.1f chosen=%s",
			chatID, len(ids), total, labelStoredUser(storedUser))
	}
	return chosen, nil
}

func (s *Sender) LocalWindow(checkInterval time.Duration) time.Duration {
	if checkInterval > s.cfg.MinumumlocalWindow {
		return checkInterval
	}
	return s.cfg.MinumumlocalWindow
}

// weightedIndex возвращает индекс по взвешенному распределению [0, total).
func weightedIndex(weights []float64, total float64) int {
	r := rand.Float64() * total
	for i, w := range weights {
		if r < w {
			return i
		}
		r -= w
	}
	return len(weights) - 1
}
