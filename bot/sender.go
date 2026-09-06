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

// Значения по умолчанию (если не заданы в Config).
const (
	defaultCheckInterval = time.Minute
	defaultMinUserWeight = 10
	defaultWeekWindow    = time.Hour * 24 * 7
)

// Параметры модели вероятности по умолчанию.
var defaultProbabilityParams = probability.Params{
	Lambda0:        1.0 / (3 * time.Hour).Seconds(),
	KMessages:      300,
	TauSilence:     (6 * time.Hour).Seconds(),
	KUserMessages:  60,
}

// Sender регулярно проверяет каждый чат: не пора ли отправить сгенерированное
// сообщение. Шанс отправки зависит от трёх пропорциональных факторов:
//   - числа сообщений в чате за окно активности (неделя);
//   - времени молчания с последнего сообщения;
//   - числа сообщений с последнего сообщения бота.
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
	applyDefaults(&cfg)

	var botID int64
	if tele.Me != nil {
		botID = tele.Me.ID
	}

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

		ticker := time.NewTicker(s.cfg.SendCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.check()
			case <-s.stop:
				return
			}
		}
	}()
}

// Close останавливает фоновую проверку и дожидается выхода из горутины.
func (s *Sender) Close() {
	close(s.stop)
	<-s.done
}

// check проходит по всем известным чатам и независимо решает про каждый.
func (s *Sender) check() {
	chats, err := s.db.GetChats()
	if err != nil {
		log.Printf("sender: get chats: %v", err)
		return
	}

	for _, chatID := range chats {
		s.maybeSend(chatID)
	}
}

func (s *Sender) maybeSend(chatID int64) {
	now := time.Now()

	ok, err := s.shouldSend(chatID, now)
	if err != nil {
		log.Printf("sender: shouldSend chat %d: %v", chatID, err)
		return
	}
	if !ok {
		return
	}

	userID, err := s.pickUser(chatID, now)
	if err != nil || userID == 0 {
		log.Printf("sender: pickUser chat %d: %v", chatID, err)
		return
	}

	text, err := s.markov.Generate(chatID, userID)
	if err != nil {
		log.Printf("sender: generate chat %d user %d: %v", chatID, userID, err)
		return
	}
	if text == "" {
		return
	}

	if err := s.send(chatID, text); err != nil {
		log.Printf("sender: send chat %d: %v", chatID, err)
		return
	}

	if err := s.db.SaveMessages([]model.Message{{
		ChatId:   chatID,
		UserId:   s.botID,
		UnixTime: now.Unix(),
	}}); err != nil {
		log.Printf("sender: save bot message chat %d: %v", chatID, err)
	}
}

func (s *Sender) send(chatID int64, text string) error {
	_, err := s.tele.Send(&tele.Chat{ID: chatID}, text)
	return err
}

// shouldSend решает, пора ли отправить сообщение в чат.
func (s *Sender) shouldSend(chatID int64, now time.Time) (bool, error) {
	windowStart := now.Add(-s.cfg.WeekWindow).Unix()

	weekCount, err := s.db.CountMessagesChat(chatID, windowStart)
	if err != nil {
		return false, err
	}

	lastActivity, err := s.db.GetLastActivityChat(chatID)
	if err != nil {
		return false, err
	}
	silence := now.Unix() - lastActivity

	// Последнее сообщение бота берём из общего кэша по userId бота.
	lastBot, err := s.db.GetLastActivityUser(chatID, s.botID)
	if err != nil {
		return false, err
	}
	msgsSinceBot, err := s.db.CountMessagesChat(chatID, lastBot+1)
	if err != nil {
		return false, err
	}

	return probability.ShouldSend(
		weekCount,
		time.Duration(silence),
		msgsSinceBot,
		s.cfg.SendCheckInterval,
		s.cfg.ProbabilityParams,
	), nil
}

// pickUser выбирает автора будущего сообщения пропорционально его активности
// за неделю, но с минимальным весом для всех участников чата. Сам бот
// исключается — он лишь подражает людям.
func (s *Sender) pickUser(chatID int64, now time.Time) (int64, error) {
	users, err := s.db.GetUsers(chatID)
	if err != nil {
		return 0, err
	}

	windowStart := now.Add(-s.cfg.WeekWindow).Unix()
	ids := make([]int64, 0, len(users))
	weights := make([]float64, 0, len(users))
	var total float64

	for _, userID := range users {
		if userID == s.botID {
			continue
		}
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
		return 0, nil
	}

	return ids[weightedIndex(weights, total)], nil
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

func applyDefaults(cfg *Config) {
	if cfg.SendCheckInterval <= 0 {
		cfg.SendCheckInterval = defaultCheckInterval
	}
	if cfg.ProbabilityParams.Lambda0 <= 0 {
		cfg.ProbabilityParams.Lambda0 = defaultProbabilityParams.Lambda0
	}
	if cfg.ProbabilityParams.KMessages <= 0 {
		cfg.ProbabilityParams.KMessages = defaultProbabilityParams.KMessages
	}
	if cfg.ProbabilityParams.TauSilence <= 0 {
		cfg.ProbabilityParams.TauSilence = defaultProbabilityParams.TauSilence
	}
	if cfg.ProbabilityParams.KUserMessages <= 0 {
		cfg.ProbabilityParams.KUserMessages = defaultProbabilityParams.KUserMessages
	}
	if cfg.MinUserWeight <= 0 {
		cfg.MinUserWeight = defaultMinUserWeight
	}
	if cfg.WeekWindow <= 0 {
		cfg.WeekWindow = defaultWeekWindow
	}
}
