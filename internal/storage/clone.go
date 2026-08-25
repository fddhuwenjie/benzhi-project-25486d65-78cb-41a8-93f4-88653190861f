package storage

import (
	"encoding/json"
	"vitrinemon/internal/model"
)

func CloneSnapshot(in model.Snapshot) model.Snapshot {
	b, err := json.Marshal(in)
	if err != nil {
		return model.NewSnapshot()
	}
	var out model.Snapshot
	if err := json.Unmarshal(b, &out); err != nil {
		return model.NewSnapshot()
	}
	return out
}
