package snapshot_nested_alias_test

import (
	"testing"
	"time"

	"vitrinemon/internal/model"
	"vitrinemon/internal/storage"
)

func TestSnapshotNestedStateIsolation(t *testing.T) {
	expectedClosedAt := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	closedAt := expectedClosedAt
	snapshot := model.NewSnapshot()
	snapshot.Batches["batch-1"] = model.InspectionBatch{
		ID:             "batch-1",
		ShowcaseIDs:    []string{"showcase-1"},
		RiskFlagCounts: map[string]int{"humidity": 1},
		LatestReasons:  []string{"湿度超限"},
		ClosedAt:       &closedAt,
	}
	snapshot.Observations["obs-1"] = model.Observation{
		ID:          "obs-1",
		BatchID:     "batch-1",
		PhotoRefs:   []string{"photo-original"},
		RiskFlags:   []string{"humidity"},
		RiskReasons: []string{"湿度超限"},
	}
	snapshot.Tasks["task-1"] = model.RemediationTask{
		ID:           "task-1",
		BatchID:      "batch-1",
		EvidenceRefs: []string{"evidence-original"},
		VerifiedAt:   &closedAt,
	}
	snapshot.Reviews["batch-1"] = []model.ReviewVersion{{Version: 1, BatchID: "batch-1", Opinion: "通过"}}

	store, err := storage.New("")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}

	returned := store.Snapshot()
	batch := returned.Batches["batch-1"]
	batch.ShowcaseIDs[0] = "tampered-showcase"
	batch.RiskFlagCounts["humidity"] = 99
	batch.LatestReasons[0] = "tampered-reason"
	*batch.ClosedAt = batch.ClosedAt.Add(24 * time.Hour)
	observation := returned.Observations["obs-1"]
	observation.PhotoRefs[0] = "tampered-photo"
	observation.RiskFlags[0] = "tampered-flag"
	observation.RiskReasons[0] = "tampered-observation-reason"
	task := returned.Tasks["task-1"]
	task.EvidenceRefs[0] = "tampered-evidence"
	*task.VerifiedAt = task.VerifiedAt.Add(24 * time.Hour)
	returned.Reviews["batch-1"][0].Opinion = "tampered-opinion"

	again := store.Snapshot()
	gotBatch := again.Batches["batch-1"]
	gotObservation := again.Observations["obs-1"]
	gotTask := again.Tasks["task-1"]
	gotReview := again.Reviews["batch-1"][0]
	if gotBatch.ShowcaseIDs[0] != "showcase-1" ||
		gotBatch.RiskFlagCounts["humidity"] != 1 ||
		gotBatch.LatestReasons[0] != "湿度超限" ||
		!gotBatch.ClosedAt.Equal(expectedClosedAt) ||
		gotObservation.PhotoRefs[0] != "photo-original" ||
		gotObservation.RiskFlags[0] != "humidity" ||
		gotObservation.RiskReasons[0] != "湿度超限" ||
		gotTask.EvidenceRefs[0] != "evidence-original" ||
		!gotTask.VerifiedAt.Equal(expectedClosedAt) ||
		gotReview.Opinion != "通过" {
		t.Fatalf("Snapshot 返回值污染了存储内部状态: batch=%+v observation=%+v task=%+v review=%+v", gotBatch, gotObservation, gotTask, gotReview)
	}

	direct := storage.CloneSnapshot(snapshot)
	directBatch := direct.Batches["batch-1"]
	directBatch.ShowcaseIDs[0] = "direct-tampered-showcase"
	directBatch.RiskFlagCounts["humidity"] = 77
	directBatch.LatestReasons[0] = "direct-tampered-reason"
	*directBatch.ClosedAt = directBatch.ClosedAt.Add(48 * time.Hour)
	directObservation := direct.Observations["obs-1"]
	directObservation.PhotoRefs[0] = "direct-tampered-photo"
	directObservation.RiskFlags[0] = "direct-tampered-flag"
	directObservation.RiskReasons[0] = "direct-tampered-observation-reason"
	directTask := direct.Tasks["task-1"]
	directTask.EvidenceRefs[0] = "direct-tampered-evidence"
	*directTask.VerifiedAt = directTask.VerifiedAt.Add(48 * time.Hour)
	direct.Reviews["batch-1"][0].Opinion = "direct-tampered-opinion"

	sourceBatch := snapshot.Batches["batch-1"]
	sourceObservation := snapshot.Observations["obs-1"]
	sourceTask := snapshot.Tasks["task-1"]
	sourceReview := snapshot.Reviews["batch-1"][0]
	if sourceBatch.ShowcaseIDs[0] != "showcase-1" ||
		sourceBatch.RiskFlagCounts["humidity"] != 1 ||
		sourceBatch.LatestReasons[0] != "湿度超限" ||
		!sourceBatch.ClosedAt.Equal(expectedClosedAt) ||
		sourceObservation.PhotoRefs[0] != "photo-original" ||
		sourceObservation.RiskFlags[0] != "humidity" ||
		sourceObservation.RiskReasons[0] != "湿度超限" ||
		sourceTask.EvidenceRefs[0] != "evidence-original" ||
		!sourceTask.VerifiedAt.Equal(expectedClosedAt) ||
		sourceReview.Opinion != "通过" {
		t.Fatalf("CloneSnapshot 返回值污染了源快照: batch=%+v observation=%+v task=%+v review=%+v", sourceBatch, sourceObservation, sourceTask, sourceReview)
	}
}
