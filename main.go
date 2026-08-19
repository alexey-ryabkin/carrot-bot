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
	config := markov.Config {
		DatabasePath:         "data/markov.db",
		Order:                3,
		DefaultMessageLength: 20,
	}

	markov, err := markov.New(config)
	if err != nil {
		log.Fatal(err)
	}

	b, err := bot.New(markov)
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
