package storage

import (
	"time"
	"vitrinemon/internal/model"
)

func CloneSnapshot(in model.Snapshot) model.Snapshot {
	out := model.NewSnapshot()
	for id, batch := range in.Batches {
		out.Batches[id] = cloneBatch(batch)
	}
	for id, observation := range in.Observations {
		out.Observations[id] = cloneObservation(observation)
	}
	for id, task := range in.Tasks {
		out.Tasks[id] = cloneTask(task)
	}
	for id, reviews := range in.Reviews {
		out.Reviews[id] = append([]model.ReviewVersion{}, reviews...)
	}
	return out
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string{}, in...)
}

func cloneIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneTimePtr(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	t := *in
	return &t
}

func cloneBatch(b model.InspectionBatch) model.InspectionBatch {
	b.ShowcaseIDs = cloneStrings(b.ShowcaseIDs)
	b.RiskFlagCounts = cloneIntMap(b.RiskFlagCounts)
	b.LatestReasons = cloneStrings(b.LatestReasons)
	b.ClosedAt = cloneTimePtr(b.ClosedAt)
	return b
}

func cloneObservation(o model.Observation) model.Observation {
	o.PhotoRefs = cloneStrings(o.PhotoRefs)
	o.RiskFlags = cloneStrings(o.RiskFlags)
	o.RiskReasons = cloneStrings(o.RiskReasons)
	o.CorrectedAt = cloneTimePtr(o.CorrectedAt)
	return o
}

func cloneTask(t model.RemediationTask) model.RemediationTask {
	t.EvidenceRefs = cloneStrings(t.EvidenceRefs)
	t.VerifiedAt = cloneTimePtr(t.VerifiedAt)
	t.EscalatedAt = cloneTimePtr(t.EscalatedAt)
	return t
}
