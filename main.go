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
		BotMessageRatio:  0.05,
		InitiativeRate:   1.5 / (8 * time.Hour).Seconds(),
		TargetWeekRate:   0.05 / 60,
		CooldownMessages: 5,
	}
	config := bot.Config{
		DatabasePath:       "data/carrotbot.db",
		LogPath:            "data/carrotbot.log",
		QueueReadInterval:  time.Second * 60,
		SendCheckInterval:  time.Second * 30,
		MinumumlocalWindow: time.Minute * 2,
		ProbabilityParams:  probabilityParams,
		MinUserWeight:      10,
		GlobalWindow:       time.Hour * 24 * 7,
	}

	configMarkov := markov.Config{
		DatabasePath: "data/markov.db",
		Order:        3,
	}

	if len(os.Args) > 1 {
		importHistory(os.Args[1], config, configMarkov)
		return
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
