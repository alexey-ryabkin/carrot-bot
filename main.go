package main

import (
	"bufio"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexey-ryabkin/carrot-bot/bot"
	"github.com/alexey-ryabkin/carrot-bot/config"
	"github.com/alexey-ryabkin/markov-module"
)

const configPath = "config.json"

func main() {
	cfg, err := config.New(configPath)
	if err != nil {
		log.Fatalf("чтение настроек %s: %v", configPath, err)
	}

	configMarkov := markov.Config{
		DatabasePath: cfg.MarkovDatabasePath(),
		Order:        cfg.MarkovOrder(),
	}

	if len(os.Args) > 1 {
		importHistory(os.Args[1], cfg, configMarkov)
		return
	}

	closeLogger, err := InitLogger(cfg.LogPath())
	if err != nil {
		log.Fatal(err)
	}
	defer closeLogger()
	log.Printf("журнал инициализирован, путь к журналу: %s", cfg.LogPath())

	log.Printf("запуск carrot-bot: database=%s markovDB=%s markovOrder=%d",
		cfg.DatabasePath(), configMarkov.DatabasePath, configMarkov.Order)

	engine, err := markov.New(&configMarkov)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("движок Маркова готов: markovOrder=%d", configMarkov.Order)

	b, err := bot.New(cfg, engine)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("бот создан")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("бот запущен, ожидание SIGINT/SIGTERM; Enter — немедленный тик")

	stopped := make(chan struct{})
	go func() {
		b.Start()
		close(stopped)
	}()

	go watchEnter(b)

	received := <-sig
	log.Printf("получен сигнал: %v", received)

	if err := b.Terminate(); err != nil {
		log.Printf("ошибка завершения: %v", err)
	}

	<-stopped
	log.Printf("работа завершена")
}

func watchEnter(b *bot.Bot) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		log.Printf("получен Enter: немедленный тик")
		b.Trigger()
	}
	if err := scanner.Err(); err != nil {
		log.Printf("чтение стандартного ввода остановлено: %v", err)
	}
}
