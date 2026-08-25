package model

type BatchSummary struct {
	BatchID          string `json:"batch_id"`
	ObservationCount int    `json:"observation_count"`
	TaskCount        int    `json:"task_count"`
	RiskLevel        string `json:"risk_level"`
	Status           string `json:"status"`
}

func SummarizeBatch(b InspectionBatch, observations []Observation, tasks []RemediationTask) BatchSummary {
	return BatchSummary{BatchID: b.ID, ObservationCount: len(observations), TaskCount: len(tasks), RiskLevel: b.RiskLevel, Status: b.Status}
}
