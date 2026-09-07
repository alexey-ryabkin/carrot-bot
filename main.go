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

	markov, err := markov.New(configMarkov)
	if err != nil {
		log.Fatal(err)
	}

	b, err := bot.New(config, markov)
	if err != nil {
		log.Fatal(err)
	}

	b.Start()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig

	if err := b.Terminate(); err != nil {
		log.Println(err)
	}
}
