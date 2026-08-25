package model

const (
	StatusCollecting = "采集中"
	StatusReviewing  = "待复核"
	StatusRectifying = "整改中"
	StatusVerifying  = "待复查"
	StatusArchived   = "已归档"
)

func NewSnapshot() Snapshot {
	return Snapshot{Batches: map[string]InspectionBatch{}, Observations: map[string]Observation{}, Tasks: map[string]RemediationTask{}, Reviews: map[string][]ReviewVersion{}}
}
