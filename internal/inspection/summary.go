package inspection

import (
	"vitrinemon/internal/model"
	"vitrinemon/internal/rules"
)

func RiskLevelOf(observations []model.Observation) string {
	level := "低"
	for _, observation := range observations {
		level = rules.HigherRisk(level, observation.RiskLevel)
	}
	return level
}
