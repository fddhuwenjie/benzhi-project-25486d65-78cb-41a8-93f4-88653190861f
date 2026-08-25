package inspection

func CanTransition(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case "采集中":
		return to == "待复核"
	case "待复核":
		return to == "整改中"
	case "整改中":
		return to == "待复查"
	case "待复查":
		return to == "已归档"
	default:
		return false
	}
}
