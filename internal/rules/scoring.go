package rules

func LevelRank(level string) int {
	switch level {
	case "高":
		return 3
	case "中":
		return 2
	default:
		return 1
	}
}

func HigherRisk(left, right string) string {
	if LevelRank(left) >= LevelRank(right) {
		return left
	}
	return right
}
