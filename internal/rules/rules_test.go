package rules

import "testing"

func TestAssessSustainedRisk(t *testing.T) {
	a := Assess("纸质", Reading{Temperature: 28, Humidity: 70, Lux: 100, DurationMinutes: 40}, nil)
	if a.Level != "高" || len(a.Flags) < 3 {
		t.Fatalf("期望高风险，得到 %#v", a)
	}
}

func TestAssessTrend(t *testing.T) {
	a := Assess("通用", Reading{Temperature: 25, Humidity: 55, Lux: 20}, []Reading{{Temperature: 20, Humidity: 40}})
	found := false
	for _, f := range a.Flags {
		if f == "trend_shift" {
			found = true
		}
	}
	if !found {
		t.Fatal("应识别趋势突变")
	}
}
