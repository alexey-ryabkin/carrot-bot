package probability

import (
	"math"
	"testing"
	"time"
)

// TestProbabilityScalesWithCheckInterval проверяет, что суммарная вероятность
// за сутки не зависит от периода проверки: дробление интервала на маленькие
// шаги даёт ту же вероятность, что и прямой расчёт за весь интервал.
func TestProbabilityScalesWithCheckInterval(t *testing.T) {
	params := Params{
		Lambda0:        1.0 / (24 * time.Hour).Seconds(),
		KMessages:      20,
		TauSilence:     (6 * time.Hour).Seconds(),
		KUserMessages:  3,
	}

	const (
		day      = 24 * time.Hour
		interval = time.Minute
	)

	silence := 3 * time.Hour
	pPerCheck := Probability(50, silence.Seconds(), 2, interval.Seconds(), params)

	checksPerDay := day / interval
	pPerDay := 1 - math.Pow(1-pPerCheck, float64(checksPerDay))

	pDirect := Probability(50, silence.Seconds(), 2, day.Seconds(), params)

	if math.Abs(pPerDay-pDirect) > 1e-9 {
		t.Fatalf("суммарная вероятность за сутки зависит от периода проверки: "+
			"через минуты = %v, напрямую = %v", pPerDay, pDirect)
	}
}
