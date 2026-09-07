package probability

import (
	"log"
	"math"
	"math/rand"
	"time"
)

// Params — параметры модели интенсивности отправки сообщений.
type Params struct {
	// Базовая интенсивность события, в событиях в секунду.
	Lambda0 float64

	// Масштаб активности чата.
	KMessages float64

	// Характерное время молчания, в секундах.
	TauSilence float64

	// Сколько сообщений пользователя нужно для насыщения
	// фактора количества сообщений.
	KUserMessages float64
}

// Probability возвращает вероятность отправки сообщения
// за интервал dt.
//
// silence в секундах.
// dt в секундах.
func Probability(
	messagesLast7Days int,
	silence float64,
	userMessages int,
	dt float64,
	p Params,
) float64 {
	if messagesLast7Days <= 0 ||
		userMessages <= 0 ||
		silence <= 0 ||
		dt <= 0 {
		return 0
	}

	// f_M = M / (M + k_M)
	messagesFactor := float64(messagesLast7Days) /
		(float64(messagesLast7Days) + p.KMessages)

	// f_S = 1 - exp(-S / tau)
	silenceFactor := 1 - math.Exp(-silence/p.TauSilence)

	// f_U = 1 - exp(-U / k_U)
	userMessagesFactor := 1 - math.Exp(-float64(userMessages)/p.KUserMessages)

	// Интенсивность события.
	lambda := p.Lambda0 *
		messagesFactor *
		silenceFactor *
		userMessagesFactor

	// P = 1 - exp(-lambda * dt)
	return 1 - math.Exp(-lambda*dt)
}

// ShouldSend решает, отправлять ли сообщение, при проверке с периодом checkInterval.
//
// messagesLast7Days — количество сообщений в чате за последние 7 дней.
// silence — время с последнего сообщения в чате, в секундах.
// userMessages — количество сообщений пользователя после последнего сообщения бота.
// checkInterval — интервал, за который рассчитывается вероятность, в секундах.
func ShouldSend(
	messagesLast7Days int,
	silence time.Duration,
	userMessages int,
	checkInterval time.Duration,
	p Params,
) bool {
	pSend := Probability(
		messagesLast7Days,
		silence.Seconds(),
		userMessages,
		checkInterval.Seconds(),
		p)

	roll := rand.Float64()
	decision := roll < pSend

	log.Printf("ShouldSend: messagesLast7Days=%d silence=%s userMessages=%d checkInterval=%v p=%.6f roll=%.6f decision=%t",
		messagesLast7Days, silence.Round(time.Second), userMessages, checkInterval, pSend, roll, decision)

	return decision
}
