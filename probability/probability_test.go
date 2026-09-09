package probability

import (
	"math"
	"testing"
	"time"
)

// testParams возвращает рекомендуемый стартовый набор параметров.
func testParams() Params {
	return Params{
		BotMessageRatio: 0.05,
		InitiativeRate:  1.5 / (8 * time.Hour).Seconds(), // ≈ 0.00005208
		TargetWeekRate:  0.05 / 60,                       // ≈ 504 сообщения/неделю → 100% живости
	}
}

// weekMessages задаёт недельную историю, заведомо достаточную
// для weekActivity = 1 при тестовых параметрах.
const weekMessages = 40000

// TestProbabilityInvalidInput проверяет защиту от некорректных аргументов.
func TestProbabilityInvalidInput(t *testing.T) {
	p := testParams()

	cases := []struct {
		name          string
		messagesLocal int
		messagesLast7 int
		dt            float64
		localWindow   float64
	}{
		{"messagesLocal < 0", -1, weekMessages, 30, 120},
		{"messagesLast7Days < 0", 0, -1, 30, 120},
		{"dt = 0", 0, weekMessages, 0, 120},
		{"dt < 0", 0, weekMessages, -30, 120},
		{"dt = 0", 0, weekMessages, 0, 120},
		{"dt < 0", 0, weekMessages, -30, 120},
		{"нет недельной активности", 0, 0, 30, 120},
		{"localWindow = 0", 100, weekMessages, 30, 0},
		{"localWindow < 0", 120, weekMessages, 30, -120},
	}

	for _, tc := range cases {
		if got := Probability(tc.messagesLocal, tc.messagesLast7, tc.dt, tc.localWindow, p); got != 0 {
			t.Errorf("%s: хотелось 0, получили %v", tc.name, got)
		}
	}
}

// TestProbabilityZeroParams проверяет, что нулевой набор параметров
// не даёт NaN и возвращает нулевую вероятность.
func TestProbabilityZeroParams(t *testing.T) {
	var p Params

	if got := Probability(0, weekMessages, 30, 120, p); got != 0 {
		t.Fatalf("нулевые параметры, тишина: хотелось 0, получили %v", got)
	}
	if got := Probability(5, weekMessages, 30, 120, p); got != 0 {
		t.Fatalf("нулевые параметры, активность: хотелось 0, получили %v", got)
	}
}

// TestProbabilityInitiativeCalibration проверяет калибровку инициативы:
// за 8 часов полной тишины в достаточно активном чате ожидается
// примерно 1.5 сообщения, то есть P = 1 - exp(-1.5).
func TestProbabilityInitiativeCalibration(t *testing.T) {
	p := testParams()

	pSend := Probability(0, weekMessages, (8 * time.Hour).Seconds(), 0, p)
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

	pPerCheck := Probability(0, weekMessages, interval.Seconds(), 120, p)

	checksPerDay := day / interval
	pPerDay := 1 - math.Pow(1-pPerCheck, float64(checksPerDay))

	pDirect := Probability(0, weekMessages, day.Seconds(), 120, p)

	if math.Abs(pPerDay-pDirect) > 1e-9 {
		t.Fatalf("суммарная вероятность за сутки зависит от периода проверки: "+
			"через минуты = %v, напрямую = %v", pPerDay, pDirect)
	}
}

// TestProbabilityLocalActivity проверяет, что вероятность растёт,
// когда люди пишут прямо сейчас.
func TestProbabilityLocalActivity(t *testing.T) {
	p := testParams()

	silent := Probability(0, weekMessages, 30, 30, p)
	active := Probability(10, weekMessages, 30, 30, p)

	if active <= silent {
		t.Fatalf("при локальной активности людей вероятность не выросла: "+
			"тишина = %v, активность = %v", silent, active)
	}
}

// TestProbabilityWeekActivity проверяет зависимость инициативы от недельной
// активности чата и её насыщение на уровне weekActivity = 1.
func TestProbabilityWeekActivity(t *testing.T) {
	p := testParams()

	low := Probability(0, 50, 30, 120, p)
	high := Probability(0, weekMessages, 30, 120, p)
	saturated := Probability(0, 10*weekMessages, 30, 120, p)

	if high <= low {
		t.Fatalf("инициатива не растёт с недельной активностью: "+
			"low = %v, high = %v", low, high)
	}
	if math.Abs(high-saturated) > 1e-12 {
		t.Fatalf("weekActivity не насыщается на 1: high = %v, saturated = %v",
			high, saturated)
	}
}
// 