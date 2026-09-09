package bot

import (
	"time"

	"github.com/alexey-ryabkin/carrot-bot/probability"
)

type Config struct {
	DatabasePath string
	LogPath      string

	QueueReadInterval time.Duration

	SendCheckInterval time.Duration      
	MinumumlocalWindow time.Duration
	GlobalWindow time.Duration
	ProbabilityParams probability.Params 
	MinUserWeight     float64            
	WeekWindow        time.Duration      
}
