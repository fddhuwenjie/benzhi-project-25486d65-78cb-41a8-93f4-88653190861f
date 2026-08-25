package rules

import (
	"fmt"
	"math"
	"sort"
)

type MaterialProfile struct {
	Name             string  `json:"name"`
	MinTemp          float64 `json:"min_temp"`
	MaxTemp          float64 `json:"max_temp"`
	MinHumidity      float64 `json:"min_humidity"`
	MaxHumidity      float64 `json:"max_humidity"`
	MaxLux           float64 `json:"max_lux"`
	ToleranceMinutes int     `json:"tolerance_minutes"`
}

var profiles = map[string]MaterialProfile{
	"纸质": {Name: "纸质", MinTemp: 16, MaxTemp: 24, MinHumidity: 45, MaxHumidity: 60, MaxLux: 50, ToleranceMinutes: 30},
	"纺织": {Name: "纺织", MinTemp: 16, MaxTemp: 24, MinHumidity: 45, MaxHumidity: 60, MaxLux: 50, ToleranceMinutes: 20},
	"金属": {Name: "金属", MinTemp: 15, MaxTemp: 26, MinHumidity: 35, MaxHumidity: 55, MaxLux: 200, ToleranceMinutes: 60},
	"陶瓷": {Name: "陶瓷", MinTemp: 15, MaxTemp: 27, MinHumidity: 35, MaxHumidity: 65, MaxLux: 300, ToleranceMinutes: 90},
	"通用": {Name: "通用", MinTemp: 16, MaxTemp: 26, MinHumidity: 40, MaxHumidity: 60, MaxLux: 150, ToleranceMinutes: 45},
}

// VersionedProfiles 是可追溯的规则快照；新版本只复制并调整阈值，不改变既有版本。
var VersionedProfiles = map[string]map[string]MaterialProfile{
	"env-v1.2": profiles,
	"env-v1.3": {
		"纸质": {Name: "纸质", MinTemp: 16, MaxTemp: 23, MinHumidity: 45, MaxHumidity: 58, MaxLux: 45, ToleranceMinutes: 30},
		"纺织": {Name: "纺织", MinTemp: 16, MaxTemp: 23, MinHumidity: 45, MaxHumidity: 58, MaxLux: 45, ToleranceMinutes: 20},
		"金属": {Name: "金属", MinTemp: 15, MaxTemp: 25, MinHumidity: 35, MaxHumidity: 55, MaxLux: 180, ToleranceMinutes: 60},
		"陶瓷": {Name: "陶瓷", MinTemp: 15, MaxTemp: 26, MinHumidity: 35, MaxHumidity: 65, MaxLux: 280, ToleranceMinutes: 90},
		"通用": {Name: "通用", MinTemp: 16, MaxTemp: 25, MinHumidity: 40, MaxHumidity: 60, MaxLux: 140, ToleranceMinutes: 45},
	},
}

type Reading struct {
	Temperature, Humidity, Lux float64
	DurationMinutes            int
}
type Assessment struct {
	Level         string             `json:"level"`
	Flags         []string           `json:"flags"`
	Reasons       []string           `json:"reasons"`
	RuleVersion   string             `json:"rule_version"`
	TriggerValues map[string]float64 `json:"trigger_values"`
}

func Profile(material string) MaterialProfile {
	if p, ok := profiles[material]; ok {
		return p
	}
	return profiles["通用"]
}

func ProfileVersion(material, version string) (MaterialProfile, bool) {
	ps, ok := VersionedProfiles[version]
	if !ok {
		return MaterialProfile{}, false
	}
	p, ok := ps[material]
	if !ok {
		p, ok = ps["通用"]
	}
	return p, ok
}

// HasProfile 用于建立批次时拒绝未定义材质，避免后续阈值漂移。
func HasProfile(material string) bool { _, ok := profiles[material]; return ok }

func Assess(material string, r Reading, previous []Reading) Assessment {
	return AssessVersion(material, "env-v1.2", r, previous)
}

func AssessVersion(material, version string, r Reading, previous []Reading) Assessment {
	p, ok := ProfileVersion(material, version)
	if !ok {
		p = Profile(material)
		version = "env-v1.2"
	}
	a := Assessment{Level: "低", RuleVersion: version, TriggerValues: map[string]float64{}}
	add := func(flag, reason string, value float64) {
		a.Flags = append(a.Flags, flag)
		a.Reasons = append(a.Reasons, reason)
		a.TriggerValues[flag] = value
	}
	if r.Temperature < p.MinTemp || r.Temperature > p.MaxTemp {
		add("temperature_out_of_range", fmt.Sprintf("温度 %.1f℃ 超出 %.1f~%.1f℃", r.Temperature, p.MinTemp, p.MaxTemp), r.Temperature)
	}
	if r.Humidity < p.MinHumidity || r.Humidity > p.MaxHumidity {
		add("humidity_out_of_range", fmt.Sprintf("相对湿度 %.1f%% 超出 %.1f~%.1f%%", r.Humidity, p.MinHumidity, p.MaxHumidity), r.Humidity)
	}
	if r.Lux > p.MaxLux {
		add("lux_over_limit", fmt.Sprintf("照度 %.1f lx 高于 %.1f lx 上限", r.Lux, p.MaxLux), r.Lux)
	}
	if r.DurationMinutes >= p.ToleranceMinutes && len(a.Flags) > 0 {
		a.Flags = append(a.Flags, "sustained_exceedance")
		a.Reasons = append(a.Reasons, fmt.Sprintf("超限持续 %d 分钟，超过容忍时长 %d 分钟", r.DurationMinutes, p.ToleranceMinutes))
	}
	if len(previous) > 0 {
		last := previous[len(previous)-1]
		if math.Abs(r.Humidity-last.Humidity) >= 12 || math.Abs(r.Temperature-last.Temperature) >= 4 {
			a.Flags = append(a.Flags, "trend_shift")
			a.Reasons = append(a.Reasons, "与上一读数相比出现明显趋势突变")
		}
	}
	if len(a.Flags) >= 3 {
		a.Level = "高"
	} else if len(a.Flags) > 0 {
		a.Level = "中"
	}
	sort.Strings(a.Flags)
	return a
}
