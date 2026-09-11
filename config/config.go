package config

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/alexey-ryabkin/carrot-bot/probability"
)

// ChatSettings — настройки одного чата.
type ChatSettings struct {
	ProbabilityParams probability.Params
	MinUserWeight     float64
}

// Service хранит настройки, прочитанные из JSON-файла.
type Service struct {
	mu   sync.RWMutex
	path string
	data *file
}

// New читает файл настроек и запоминает его.
func New(path string) (*Service, error) {
	s := &Service{path: path}
	if err := s.reload(); err != nil {
		return nil, err
	}
	return s, nil
}

// reload перечитывает файл. Старые настройки остаются, если новый файл
// прочитать не удалось.
func (s *Service) reload() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}

	s.mu.Lock()
	s.data = &f
	s.mu.Unlock()
	return nil
}

func (s *Service) DatabasePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.DatabasePath.Value
}

func (s *Service) LogPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.LogPath.Value
}

func (s *Service) QueueReadInterval() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return seconds(s.data.QueueReadIntervalSec.Value)
}

func (s *Service) SendCheckInterval() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return seconds(s.data.SendCheckIntervalSec.Value)
}

func (s *Service) MinimumLocalWindow() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return seconds(s.data.MinimumLocalWindowSec.Value)
}

func (s *Service) GlobalWindow() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return seconds(s.data.GlobalWindowSec.Value)
}

func (s *Service) MarkovDatabasePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Markov.DatabasePath.Value
}

func (s *Service) MarkovOrder() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int(s.data.Markov.Order.Value)
}

func (s *Service) DefaultChatSettings() ChatSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.DefaultChat.settings()
}

// SettingsFor возвращает настройки чата. Перед этим файл перечитывается,
// поэтому настройки можно менять без перезапуска бота.
func (s *Service) SettingsFor(chatID int64) ChatSettings {
	if err := s.reload(); err != nil {
		log.Printf("настройки: не удалось перечитать %s: %v", s.path, err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, chat := range s.data.Chats {
		if chat.ID.Value == chatID {
			return chat.settings()
		}
	}
	return s.data.DefaultChat.settings()
}

func seconds(value int64) time.Duration {
	return time.Duration(value) * time.Second
}

type file struct {
	DatabasePath          text       `json:"database_path"`
	LogPath               text       `json:"log_path"`
	QueueReadIntervalSec  integer    `json:"queue_read_interval_seconds"`
	SendCheckIntervalSec  integer    `json:"send_check_interval_seconds"`
	MinimumLocalWindowSec integer    `json:"minimum_local_window_seconds"`
	GlobalWindowSec       integer    `json:"global_window_seconds"`
	Markov                markovFile `json:"markov"`
	DefaultChat           chatFile   `json:"default_chat"`
	Chats                 []chatFile `json:"chats"`
}

type markovFile struct {
	DatabasePath text    `json:"database_path"`
	Order        integer `json:"order"`
}

type chatFile struct {
	Name          string          `json:"name"`
	ID            integer         `json:"id"`
	MinUserWeight number          `json:"min_user_weight"`
	Probability   probabilityFile `json:"probability"`
}

func (c chatFile) settings() ChatSettings {
	return ChatSettings{
		ProbabilityParams: probability.Params{
			BotMessageRatio:  c.Probability.BotMessageRatio.Value,
			InitiativeRate:   c.Probability.InitiativeRate.Value,
			TargetWeekRate:   c.Probability.TargetWeekRate.Value,
			CooldownMessages: c.Probability.CooldownMessages.Value,
		},
		MinUserWeight: c.MinUserWeight.Value,
	}
}

type probabilityFile struct {
	BotMessageRatio  number `json:"bot_message_ratio"`
	InitiativeRate   number `json:"initiative_rate"`
	TargetWeekRate   number `json:"target_week_rate"`
	CooldownMessages number `json:"cooldown_messages"`
}

// text — строковая настройка. Поле Description бот не использует.
type text struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

// number — числовая настройка. Поле Description бот не использует.
type number struct {
	Value       float64 `json:"value"`
	Description string  `json:"description"`
}

// integer — целочисленная настройка. Поле Description бот не использует.
type integer struct {
	Value       int64  `json:"value"`
	Description string `json:"description"`
}
