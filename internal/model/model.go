package model

import "time"

type InspectionBatch struct {
	ID                 string         `json:"id"`
	Title              string         `json:"title"`
	ShowcaseIDs        []string       `json:"showcase_ids"`
	Collector          string         `json:"collector"`
	WindowStart        time.Time      `json:"window_start"`
	WindowEnd          time.Time      `json:"window_end"`
	Status             string         `json:"status"`
	RiskLevel          string         `json:"risk_level"`
	Revision           int            `json:"revision"`
	CreatedAt          time.Time      `json:"created_at"`
	ClosedAt           *time.Time     `json:"closed_at,omitempty"`
	Material           string         `json:"material"`
	RiskFlagCounts     map[string]int `json:"risk_flag_counts,omitempty"`
	LatestReasons      []string       `json:"latest_reasons,omitempty"`
	ReviewVersion      int            `json:"review_version,omitempty"`
	ReviewOpinion      string         `json:"review_opinion,omitempty"`
	LockedRiskLevel    string         `json:"locked_risk_level,omitempty"`
	CurrentRuleVersion string         `json:"current_rule_version,omitempty"`
}
type Observation struct {
	ID                 string     `json:"id"`
	BatchID            string     `json:"batch_id"`
	ShowcaseID         string     `json:"showcase_id"`
	RecordedAt         time.Time  `json:"recorded_at"`
	TemperatureCelsius float64    `json:"temperature_celsius"`
	RelativeHumidity   float64    `json:"relative_humidity"`
	Lux                float64    `json:"lux"`
	DurationMinutes    int        `json:"duration_minutes"`
	Notes              string     `json:"notes"`
	PhotoRefs          []string   `json:"photo_refs"`
	RiskFlags          []string   `json:"risk_flags"`
	RiskReasons        []string   `json:"risk_reasons"`
	RiskLevel          string     `json:"risk_level"`
	RuleVersion        string     `json:"rule_version"`
	Revoked            bool       `json:"revoked,omitempty"`
	OriginalSummary    string     `json:"original_summary,omitempty"`
	CorrectionReason   string     `json:"correction_reason,omitempty"`
	CorrectedAt        *time.Time `json:"corrected_at,omitempty"`
}
type RemediationTask struct {
	ID                 string     `json:"id"`
	BatchID            string     `json:"batch_id"`
	Finding            string     `json:"finding"`
	Assignee           string     `json:"assignee"`
	DueAt              time.Time  `json:"due_at"`
	AcceptanceCriteria string     `json:"acceptance_criteria"`
	ActionResult       string     `json:"action_result"`
	EvidenceRefs       []string   `json:"evidence_refs"`
	Status             string     `json:"status"`
	VerifiedBy         string     `json:"verified_by"`
	VerifiedAt         *time.Time `json:"verified_at,omitempty"`
	EscalatedAt        *time.Time `json:"escalated_at,omitempty"`
}

type ReviewVersion struct {
	Version     int       `json:"version"`
	BatchID     string    `json:"batch_id"`
	Opinion     string    `json:"opinion"`
	RiskLevel   string    `json:"risk_level"`
	RuleVersion string    `json:"rule_version"`
	Revision    int       `json:"revision"`
	Actor       string    `json:"actor"`
	CreatedAt   time.Time `json:"created_at"`
	Dispute     string    `json:"dispute,omitempty"`
}
type AuditEvent struct {
	ID          string    `json:"id"`
	BatchID     string    `json:"batch_id"`
	EventType   string    `json:"event_type"`
	Actor       string    `json:"actor"`
	OccurredAt  time.Time `json:"occurred_at"`
	RequestID   string    `json:"request_id"`
	FromStatus  string    `json:"from_status"`
	ToStatus    string    `json:"to_status"`
	PayloadHash string    `json:"payload_hash"`
	PrevHash    string    `json:"prev_hash,omitempty"`
	Summary     string    `json:"summary"`
}
type Snapshot struct {
	Batches      map[string]InspectionBatch `json:"batches"`
	Observations map[string]Observation     `json:"observations"`
	Tasks        map[string]RemediationTask `json:"tasks"`
	Reviews      map[string][]ReviewVersion `json:"reviews,omitempty"`
}
