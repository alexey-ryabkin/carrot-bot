package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexey-ryabkin/carrot-bot/bot"
	"github.com/alexey-ryabkin/carrot-bot/probability"
	"github.com/alexey-ryabkin/markov-module"
)

func main() {
	var probabilityParams = probability.Params{
		Lambda0:       1.0 / (3 * time.Hour).Seconds(),
		KMessages:     300,
		TauSilence:    (6 * time.Hour).Seconds(),
		KUserMessages: 60,
	}
	config := bot.Config{
		DatabasePath:      "data/carrotbot.db",
		LogPath:           "data/carrotbot.log",
		QueueReadInterval: time.Second * 60,
		SendCheckInterval: time.Second * 30,
		ProbabilityParams: probabilityParams,
		MinUserWeight:     10,
		WeekWindow:        time.Hour * 24 * 7,
	}
	var testingProbabilityParams = probability.Params{
		Lambda0:       100 / (3 * time.Second).Seconds(),
		KMessages:     300,
		TauSilence:    (6 * time.Second).Seconds(),
		KUserMessages: 60,
	}
	testingConfig := bot.Config{
		DatabasePath:      "data/carrotbot.db",
		LogPath:           "data/carrotbot.log",
		QueueReadInterval: time.Second * 4,
		SendCheckInterval: time.Second * 2,
		ProbabilityParams: testingProbabilityParams,
		MinUserWeight:     10,
		WeekWindow:        time.Hour * 24 * 7,
	}
	config = testingConfig

	configMarkov := markov.Config{
		DatabasePath: "data/markov.db",
		Order:        3,
	}

	closeLogger, err := InitLogger(config.LogPath)
	if err != nil {
		log.Fatal(err)
	}
	defer closeLogger()
	log.Printf("журнал инициализирован, путь к журналу: %s", config.LogPath)

	log.Printf("запуск carrot-bot: database=%s markovDB=%s markovOrder=%d",
		config.DatabasePath, configMarkov.DatabasePath, configMarkov.Order)

	engine, err := markov.New(&configMarkov)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("движок Маркова готов: markovOrder=%d", configMarkov.Order)

	b, err := bot.New(config, engine)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("бот создан")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("бот запущен, ожидание SIGINT/SIGTERM")

	stopped := make(chan struct{})
	go func() {
		b.Start()
		close(stopped)
	}()

	received := <-sig
	log.Printf("получен сигнал: %v", received)

	if err := b.Terminate(); err != nil {
		log.Printf("ошибка завершения: %v", err)
	}

	<-stopped
	log.Printf("работа завершена")
}
