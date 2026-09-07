package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexey-ryabkin/carrot-bot/bot"
	"github.com/alexey-ryabkin/markov-module"
)

func main() {
	config := bot.Config{
		DatabasePath: "data/carrotbot.db",
		LogPath:      "data/carrotbot.log",
	}

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

	b.Start()
	log.Printf("бот запущен, ожидание SIGINT/SIGTERM")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	received := <-sig
	log.Printf("получен сигнал: %v", received)

	if err := b.Terminate(); err != nil {
		log.Printf("ошибка завершения: %v", err)
	}
	log.Printf("работа завершена")
}
