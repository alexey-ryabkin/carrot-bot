package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexey-ryabkin/carrot-bot/bot"
)

func main() {
	b, err := bot.New()
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
