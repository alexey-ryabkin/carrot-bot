package bot

import (
	"time"

	"github.com/alexey-ryabkin/carrot-bot/probability"
)

type Config struct {
	DatabasePath string

	// Настройки регулярной проверки отправки сообщений.
	SendCheckInterval time.Duration      // период проверки чатов
	ProbabilityParams probability.Params // параметры модели вероятности
	MinUserWeight     float64            // минимальный вес пользователя при выборе
	WeekWindow        time.Duration      // окно активности (по плану — неделя)
}
