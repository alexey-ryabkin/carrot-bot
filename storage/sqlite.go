package storage

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/alexey-ryabkin/carrot-bot/model"
)

type SQLite struct {
	db *sql.DB
}

func (s *SQLite) GetChats() ([]int64, error) {
	var chats []int64

	rows, err := s.db.Query(`
		SELECT DISTINCT chatId
		FROM messages
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var chatId int64

		if err := rows.Scan(&chatId); err != nil {
			return nil, err
		}

		chats = append(chats, chatId)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chats, nil
}

func (s *SQLite) GetUsers(chatId int64) ([]int64, error) {
	var users []int64

	rows, err := s.db.Query(`
		SELECT DISTINCT userId
		FROM messages
		WHERE chatId = ?
	`, chatId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var userId int64

		if err := rows.Scan(&userId); err != nil {
			return nil, err
		}

		users = append(users, userId)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (s *SQLite) Close() error {
	log.Printf("закрытие базы данных")
	return s.db.Close()
}

// Batch processing

func (s *SQLite) CleanOldMessages(d time.Duration) error {
	cutoff := time.Now().Add(-d).Unix()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		DELETE FROM messages
		WHERE unixtime <= ?
	`, cutoff)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	deleted, err := res.RowsAffected()
	if err != nil {
		log.Printf("удаление выполнено, но число затронутых строк недоступно: %v", err)
		return nil
	}
	log.Printf("удалено старых сообщений: %d", deleted)
	return nil
}

func (s *SQLite) UpsertUser(user model.User) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO users (id, firstName, lastName, username)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			firstName = excluded.firstName,
			lastName = excluded.lastName,
			username = excluded.username
	`, user.ID, user.FirstName, user.LastName, user.Username)

	return tx.Commit()
}

func (s *SQLite) GetUser(id int64) (model.User, error) {
	var user model.User
	err := s.db.QueryRow(`
		SELECT id, firstName, lastName, username
		FROM users
		WHERE id = ?
	`, id).Scan(&user.ID, &user.FirstName, &user.LastName, &user.Username)
	return user, err
}

func (s *SQLite) UpsertChat(chat model.Chat) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO chats (id, type, title, username)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type = excluded.type,
			title = excluded.title,
			username = excluded.username
	`, chat.ID, chat.Type, chat.Title, chat.Username)

	return tx.Commit()
}

func (s *SQLite) GetChat(id int64) (model.Chat, error) {
	var chat model.Chat
	err := s.db.QueryRow(`
		SELECT id, type, title, username
		FROM chats
		WHERE id = ?
	`, id).Scan(&chat.ID, &chat.Type, &chat.Title, &chat.Username)
	return chat, err
}

func (s *SQLite) SaveMessages(messages []model.Message) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmtMessages, err := tx.Prepare(`
		INSERT INTO messages (chatId, userId, unixtime)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmtMessages.Close()

	for _, message := range messages {
		_, err = stmtMessages.Exec(
			message.ChatId,
			message.UserId,
			message.UnixTime)
		if err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	log.Printf("сохранено сообщений: %d", len(messages))
	return nil

}

// Создание и подключение

func GetDB(path string) (*SQLite, error) {
	log.Printf("открытие базы данных sqlite: %s", path)

	var s *SQLite

	dir := filepath.Dir(path)

	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)

	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	s = &SQLite{
		db: db,
	}

	if err = s.initialize(); err != nil {
		s.db.Close()
		return nil, err
	}

	log.Printf("база данных готова: %s", path)
	return s, nil
}

func (s *SQLite) initialize() error {
	queries := []string{
		`PRAGMA foreign_keys = ON;`,

		`
		CREATE TABLE IF NOT EXISTS metadata (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			userVersion INTEGER NOT NULL
		)
		`,

		`
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY,
			chatId INTEGER NOT NULL,
			userId INTEGER NOT NULL,
			unixtime INTEGER NOT NULL
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_messages_chat_user
		ON messages(chatId, userId, unixtime)
		`,

		`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			firstName TEXT NOT NULL DEFAULT '',
			lastName TEXT NOT NULL DEFAULT '',
			username TEXT NOT NULL DEFAULT ''
		)
		`,

		`
		CREATE TABLE IF NOT EXISTS chats (
			id INTEGER PRIMARY KEY,
			type TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			username TEXT NOT NULL DEFAULT ''
		)
		`,
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, q := range queries {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

// Утилиты

func (s *SQLite) CountMessagesUser(chatId int64, userId int64, unixtime int64) (int, error) {
	var count int

	err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM messages
		WHERE chatId = ?
		  AND userId = ?
		  AND unixtime >= ?
	`,
		chatId,
		userId,
		unixtime,
	).Scan(&count)

	return count, err
}

func (s *SQLite) CountMessagesChat(chatId int64, unixtime int64) (int, error) {
	var count int

	err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM messages
		WHERE chatId = ?
		  AND unixtime >= ?
	`,
		chatId,
		unixtime,
	).Scan(&count)

	return count, err
}

// CountMessagesPeople считает сообщения людей в чате за период от unixtime,
// исключая заданного пользователя (например, самого бота, чьи сообщения
// тоже пишутся в кэш активности).
func (s *SQLite) CountMessagesPeople(chatId int64, excludeUserId int64, unixtime int64) (int, error) {
	var count int

	err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM messages
		WHERE chatId = ?
		  AND userId <> ?
		  AND unixtime >= ?
	`,
		chatId,
		excludeUserId,
		unixtime,
	).Scan(&count)

	return count, err
}

func (s *SQLite) GetLastActivityChat(chatId int64) (unixtime int64, err error) {
	err = s.db.QueryRow(`
		SELECT COALESCE(MAX(unixtime), 0)
		FROM messages
		WHERE chatId = ?
	`,
		chatId,
	).Scan(&unixtime)

	return unixtime, err
}

func (s *SQLite) GetLastActivityUser(chatId int64, userId int64) (unixitme int64, err error) {
	err = s.db.QueryRow(`
		SELECT COALESCE(MAX(unixtime), 0)
		FROM messages
		WHERE chatId = ?
		  AND userId = ?
	`,
		chatId,
		userId,
	).Scan(&unixitme)

	return unixitme, err
}
