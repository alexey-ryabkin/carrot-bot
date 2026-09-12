package main

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/alexey-ryabkin/carrot-bot/config"
	"github.com/alexey-ryabkin/carrot-bot/model"
	"github.com/alexey-ryabkin/carrot-bot/storage"
	"github.com/alexey-ryabkin/markov-module"
	markovModel "github.com/alexey-ryabkin/markov-module/model"
)

// Структуры Telegram-экспорта. Оставлены только нужные боту поля.

type jsonChat struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	ID       int64         `json:"id"`
	Messages []jsonMessage `json:"messages"`
}

type jsonFile struct {
	Chats struct {
		List []jsonChat `json:"list"`
	} `json:"chats"`
	LeftChats struct {
		List []jsonChat `json:"list"`
	} `json:"left_chats"`
	jsonChat
}

type jsonMessage struct {
	Type     string   `json:"type"`
	From     string   `json:"from"`
	FromID   string   `json:"from_id"`
	UnixTime string   `json:"date_unixtime"`
	Text     jsonText `json:"text"`
}

// jsonText умеет читать text, который в экспорте бывает строкой или списком частей.
type jsonText string

func (t *jsonText) UnmarshalJSON(b []byte) error {
	if b[0] != '[' {
		var s string
		json.Unmarshal(b, &s)
		*t = jsonText(s)
		return nil
	}

	var parts []json.RawMessage
	if err := json.Unmarshal(b, &parts); err != nil {
		return err
	}

	var sb strings.Builder
	for _, part := range parts {
		var s string
		if json.Unmarshal(part, &s) == nil {
			sb.WriteString(s)
			continue
		}
		var item struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(part, &item) == nil {
			sb.WriteString(item.Text)
		}
	}
	*t = jsonText(sb.String())
	return nil
}

func parseUserID(fromID string) int64 {
	id, _ := strconv.ParseInt(strings.TrimPrefix(fromID, "user"), 10, 64)
	return id
}

func importHistory(path string, cfg *config.Service, markovCfg markov.Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	var file jsonFile
	if err := json.Unmarshal(data, &file); err != nil {
		log.Fatal(err)
	}

	chats := file.Chats.List
	if len(chats) == 0 {
		chats = file.LeftChats.List
	}
	if len(chats) == 0 {
		chats = []jsonChat{file.jsonChat}
	}

	engine, err := markov.New(&markovCfg)
	if err != nil {
		log.Fatal(err)
	}
	db, err := storage.GetDB(cfg.DatabasePath())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	total := 0
	for _, chat := range chats {
		c := model.Chat{ID: chat.ID, Type: chat.Type, Title: chat.Name}
		if err := db.UpsertChat(c); err != nil {
			log.Fatal(err)
		}

		users := map[int64]model.User{}
		var markovMessages []markovModel.Message
		var activity []model.Message

		for _, m := range chat.Messages {
			if m.Type != "message" {
				continue
			}
			userID := parseUserID(m.FromID)
			if userID == 0 {
				continue
			}
			unixTime, err := strconv.ParseInt(m.UnixTime, 10, 64)
			if err != nil {
				continue
			}

			users[userID] = model.User{ID: userID, FirstName: m.From}
			if m.Text != "" {
				markovMessages = append(markovMessages, markovModel.Message{
					ChatId:   chat.ID,
					UserId:   userID,
					UnixTime: unixTime,
					Text:     string(m.Text),
				})
			}
			activity = append(activity, model.Message{
				ChatId:   chat.ID,
				UserId:   userID,
				UnixTime: unixTime,
			})
		}

		for _, u := range users {
			if err := db.UpsertUser(u, c); err != nil {
				log.Fatal(err)
			}
		}

		if err := engine.LearnMany(markovMessages); err != nil {
			log.Fatal(err)
		}
		if err := db.SaveMessages(activity); err != nil {
			log.Fatal(err)
		}
		total += len(activity)
		log.Printf("чат %q импортирован: %d сообщений, %d пользователей", chat.Name, len(activity), len(users))
	}

	log.Printf("импорт завершён: чатов=%d сообщений=%d", len(chats), total)
}
