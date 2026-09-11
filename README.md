# carrot-bot

## Подготовка к запуску

Установить Go: https://go.dev/dl/

```
git clone https://github.com/alexey-ryabkin/carrot-bot.git
cd carrot-bot
go install
cp config_example.json config.json
```

Описание настроек — в `config.json`.

## Запуск

```
export TELEGRAM_TOKEN=bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11
carrot-bot
```

Остановка: `Ctrl+C`

Переменную среды можно добавить в `~/.bashrc`

## Импорт истории сообщений
Скачать историю сообщений (файл result.json) из desktop приложения Telegram.

```
carrot-bot result.json
```

ID чата в `result.json` может не совпадать с фактическим - необходимо до импорта запустить бота один раз, посмотреть id в логе и исправить id в самом начале json.
