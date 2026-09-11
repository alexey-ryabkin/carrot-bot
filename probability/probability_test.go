package probability

import (
	"math"
	"testing"
	"time"
)

// testParams возвращает рекомендуемый стартовый набор параметров.
func testParams() Params {
	return Params{
		BotMessageRatio:  0.05,
		InitiativeRate:   1.5 / (8 * time.Hour).Seconds(), // ≈ 0.00005208
		TargetWeekRate:   0.05 / 60,                       // ≈ 504 сообщения/неделю → 100% живости
		CooldownMessages: 10,                              // полный шанс после 10 сообщений людей
	}
}

// weekMessages задаёт недельную историю, заведомо достаточную
// для weekActivity = 1 при тестовых параметрах.
const weekMessages = 40000

// weekSeconds — длительность недельного окна в секундах (совпадает
// с GlobalWindow в bot.Config).
const weekSeconds = 7 * 24 * 3600.0

// TestProbabilityInvalidInput проверяет защиту от некорректных аргументов.
func TestProbabilityInvalidInput(t *testing.T) {
	p := testParams()

	cases := []struct {
		name            string
		messagesLocal   int
		messagesLast7   int
		messagesSinceBot int
		localWindow     float64
		globalWindow    float64
		dt              float64
	}{
		{"messagesLocal < 0", -1, weekMessages, 5, 120, weekSeconds, 30},
		{"messagesLast7Days < 0", 0, -1, 5, 120, weekSeconds, 30},
		{"messagesSinceBot < 0", 0, weekMessages, -1, 120, weekSeconds, 30},
		{"dt = 0", 0, weekMessages, 5, 120, weekSeconds, 0},
		{"dt < 0", 0, weekMessages, 5, 120, weekSeconds, -30},
		{"нет недельной активности", 0, 0, 5, 120, weekSeconds, 30},
		{"localWindow = 0", 100, weekMessages, 5, 0, weekSeconds, 30},
		{"localWindow < 0", 120, weekMessages, 5, -120, weekSeconds, 30},
		{"globalWindow = 0", 0, weekMessages, 5, 120, 0, 30},
		{"globalWindow < 0", 0, weekMessages, 5, 120, -weekSeconds, 30},
	}

	for _, tc := range cases {
		got := Probability(
			tc.messagesLocal,
			tc.messagesLast7,
			tc.messagesSinceBot,
			tc.localWindow,
			tc.globalWindow,
			tc.dt,
			p,
		)
		if got != 0 {
			t.Errorf("%s: хотелось 0, получили %v", tc.name, got)
		}
	}
}

// TestProbabilityZeroParams проверяет, что нулевой набор параметров
// не даёт NaN и возвращает нулевую вероятность.
func TestProbabilityZeroParams(t *testing.T) {
	var p Params

	if got := Probability(0, weekMessages, 5, 120, weekSeconds, 30, p); got != 0 {
		t.Fatalf("нулевые параметры, тишина: хотелось 0, получили %v", got)
	}
	if got := Probability(5, weekMessages, 5, 120, weekSeconds, 30, p); got != 0 {
		t.Fatalf("нулевые параметры, активность: хотелось 0, получили %v", got)
	}
}

// TestProbabilityInitiativeCalibration проверяет калибровку инициативы:
// за 8 часов полной тишины в достаточно активном чате ожидается
// примерно 1.5 сообщения, то есть P = 1 - exp(-1.5).
func TestProbabilityInitiativeCalibration(t *testing.T) {
	p := testParams()

	pSend := Probability(0, weekMessages, 10, 120, weekSeconds, (8*time.Hour).Seconds(), p)
	expected := 1 - math.Exp(-1.5)

	if math.Abs(pSend-expected) > 1e-6 {
		t.Fatalf("инициативная вероятность за 8 часов: получили %v, хотелось %v", pSend, expected)
	}
}

// TestProbabilityScalesWithCheckInterval проверяет, что суммарная
// вероятность за сутки полной тишины не зависит от периода проверки:
// дробление интервала на маленькие шаги даёт ту же вероятность,
// что и прямой расчёт за весь интервал.
func TestProbabilityScalesWithCheckInterval(t *testing.T) {
	p := testParams()

	const (
		day      = 24 * time.Hour
		interval = time.Minute
	)

	pPerCheck := Probability(0, weekMessages, 10, 120, weekSeconds, interval.Seconds(), p)

	checksPerDay := day / interval
	pPerDay := 1 - math.Pow(1-pPerCheck, float64(checksPerDay))

	pDirect := Probability(0, weekMessages, 10, 120, weekSeconds, day.Seconds(), p)

	if math.Abs(pPerDay-pDirect) > 1e-9 {
		t.Fatalf("суммарная вероятность за сутки зависит от периода проверки: "+
			"через минуты = %v, напрямую = %v", pPerDay, pDirect)
	}
}

// TestProbabilityLocalActivity проверяет, что вероятность растёт,
// когда люди пишут прямо сейчас.
func TestProbabilityLocalActivity(t *testing.T) {
	p := testParams()

	silent := Probability(0, weekMessages, 10, 30, weekSeconds, 30, p)
	active := Probability(10, weekMessages, 10, 30, weekSeconds, 30, p)

	if active <= silent {
		t.Fatalf("при локальной активности людей вероятность не выросла: "+
			"тишина = %v, активность = %v", silent, active)
	}
}

// TestProbabilityWeekActivity проверяет зависимость инициативы от недельной
// активности чата и её насыщение на уровне weekActivity = 1.
func TestProbabilityWeekActivity(t *testing.T) {
	p := testParams()

	low := Probability(0, 50, 10, 120, weekSeconds, 30, p)
	high := Probability(0, weekMessages, 10, 120, weekSeconds, 30, p)
	saturated := Probability(0, 10*weekMessages, 10, 120, weekSeconds, 30, p)

	if high <= low {
		t.Fatalf("инициатива не растёт с недельной активностью: "+
			"low = %v, high = %v", low, high)
	}
	if math.Abs(high-saturated) > 1e-12 {
		t.Fatalf("weekActivity не насыщается на 1: high = %v, saturated = %v",
			high, saturated)
	}
}

// TestProbabilityCooldown проверяет душащий коэффициент: сразу после
// сообщения бота шанс равен нулю, при половине порога коэффициент равен 0.5,
// а при достижении порога (и дальше) — равен 1.
func TestProbabilityCooldown(t *testing.T) {
	p := testParams()
	threshold := int(p.CooldownMessages)

	immediate := Probability(0, weekMessages, 0, 120, weekSeconds, 30, p)
	if math.Abs(immediate-0.1) > 1e-9 {
		t.Fatalf("сразу после сообщения бота вероятность должна быть 0.1, получили %v", immediate)
	}

	full := Probability(0, weekMessages, threshold, 120, weekSeconds, 30, p)

	// При половине порога коэффициент = 0.5, то есть 1-P = sqrt(1-P_full).
	half := Probability(0, weekMessages, threshold/2, 120, weekSeconds, 30, p)
	expectedHalf := 1 - math.Sqrt(1-full)
	if math.Abs(half-expectedHalf) > 1e-9 {
		t.Fatalf("при половине порога коэффициент не 0.5: получили %v, хотелось %v",
			half, expectedHalf)
	}

	// При достижении порога коэффициент = 1 и больше не растёт.
	saturated := Probability(0, weekMessages, 10*threshold, 120, weekSeconds, 30, p)
	if math.Abs(full-saturated) > 1e-12 {
		t.Fatalf("коэффициент не насыщается на 1 при пороге: full = %v, saturated = %v",
			full, saturated)
	}
}
