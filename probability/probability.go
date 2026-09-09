package probability

import (
	"log"
	"math"
	"math/rand"
	"time"
)

// Params — параметры модели интенсивности отправки сообщений.
type Params struct {
	// BotMessageRatio — доля сообщений бота от локальной активности людей.
	//
	// 0.01 = ~1 сообщение бота на 100 сообщений людей
	// 0.02 = ~1 на 50
	// 0.05 = ~1 на 20
	// 0.10 = ~1 на 10
	BotMessageRatio float64

	// InitiativeRate — базовая интенсивность самостоятельных сообщений
	// бота в период полной тишины, событий в секунду.
	//
	// Для ~1.5 сообщения за 8 часов:
	// 1.5 / (8 * 3600) ≈ 0.00005208.
	InitiativeRate float64

	// TargetWeekRate — скорость сообщений в секунду, соответствующая 100%
	// "живости" чата по недельной истории: weekActivity =
	// min(weekRate / TargetWeekRate, 1).
	//
	// Рекомендуемый старт: 0.05 / 60 ≈ 0.00083 сообщений/сек, то есть
	// живость 100% достигается при ~504 сообщениях людей за неделю
	// (0.05 сообщений/мин).
	TargetWeekRate float64
}

// Probability возвращает вероятность отправки сообщения за интервал dt.
//
// messagesLocal — количество сообщений людей в чате за последние
// min(dt, 2 минуты) секунд.
// messagesLast7Days — количество сообщений людей в чате за последние 7 дней.
// dt — интервал, за который считается вероятность, в секундах.
func Probability(
	messagesLocal int,
	messagesLast7Days int,
	localWindow float64,
	globalWindow float64,
	dt float64,
	p Params,
) float64 {
	if messagesLocal < 0 ||
		messagesLast7Days < 0 ||
		dt <= 0 {
		return 0
	}
	localRate := float64(messagesLocal) / localWindow
	weekRate := float64(messagesLast7Days) / globalWindow

	weekActivity := 0.0
	if p.TargetWeekRate > 0 {
		weekActivity = math.Min(weekRate/p.TargetWeekRate, 1)
	}

	var lambda float64
	if messagesLocal == 0 {
		lambda = p.InitiativeRate * weekActivity
	} else {
		lambda = p.BotMessageRatio * localRate
	}

	return 1 - math.Exp(-lambda*dt)
}

// ShouldSend решает, отправлять ли сообщение, при проверке с периодом
// checkInterval.
//
// messagesLocal — количество сообщений людей в чате за последние
// min(checkInterval, 2 минуты) секунд.
// messagesLast7Days — количество сообщений людей в чате за последние 7 дней.
func ShouldSend(
	messagesLocal int,
	messagesLast7Days int,
	localWindow time.Duration,
	globalWindow time.Duration,
	checkInterval time.Duration,
	p Params,
) bool {
	pSend := Probability(
		messagesLocal,
		messagesLast7Days,
		localWindow.Seconds(),
		globalWindow.Seconds(),
		checkInterval.Seconds(),
		p)

	roll := rand.Float64()
	decision := roll < pSend

	log.Printf("ShouldSend: messagesLocal=%d messagesLast7Days=%d checkInterval=%v p=%.6f roll=%.6f decision=%t",
		messagesLocal, messagesLast7Days, checkInterval, pSend, roll, decision)

	return decision
}
