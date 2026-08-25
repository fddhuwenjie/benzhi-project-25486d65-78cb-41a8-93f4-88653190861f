package storage

import (
	"vitrinemon/internal/model"
)

func CloneSnapshot(in model.Snapshot) model.Snapshot {
	out := model.NewSnapshot()
	for id, batch := range in.Batches {
		out.Batches[id] = batch
	}
	for id, observation := range in.Observations {
		out.Observations[id] = observation
	}
	for id, task := range in.Tasks {
		out.Tasks[id] = task
	}
	for id, reviews := range in.Reviews {
		out.Reviews[id] = reviews
	}
	return out
}
