package bot

import (
	"time"

	"github.com/alexey-ryabkin/carrot-bot/probability"
)

type ChatSettings struct {
	ProbabilityParams probability.Params
	MinUserWeight     float64
}

type Config struct {
	DatabasePath string
	LogPath      string

	QueueReadInterval  time.Duration
	SendCheckInterval  time.Duration
	MinumumlocalWindow time.Duration
	GlobalWindow       time.Duration

	// Ключ — id чата.
	ChatSettings map[int64]ChatSettings

	DefaultChatSettings ChatSettings
}

func (c Config) SettingsFor(chatID int64) ChatSettings {
	if settings, ok := c.ChatSettings[chatID]; ok {
		return settings
	}
	return c.DefaultChatSettings
}
