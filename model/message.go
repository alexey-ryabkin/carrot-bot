package model

type Message struct {
	ChatId   int64
	UserId   int64
	UnixTime int64
}

type User struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
}

type Chat struct {
	ID       int64
	Type     string
	Title    string
	Username string
}
