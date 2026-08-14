module github.com/alexey-ryabkin/carrot-bot

go 1.25.0

require gopkg.in/telebot.v3 v3.3.8 // indirect

require (
	github.com/alexey-ryabkin/markov-module v0.0.0
	gopkg.in/telebot.v4 v4.0.0-beta.10
)

replace github.com/alexey-ryabkin/markov-module => ../markov-module
