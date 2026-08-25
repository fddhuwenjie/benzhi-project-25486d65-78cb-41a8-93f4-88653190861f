package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"vitrinemon/internal/model"
)

type Store struct {
	mu   sync.RWMutex
	path string
	data model.Snapshot
}

func New(path string) (*Store, error) {
	s := &Store{path: path, data: model.Snapshot{Batches: map[string]model.InspectionBatch{}, Observations: map[string]model.Observation{}, Tasks: map[string]model.RemediationTask{}, Reviews: map[string][]model.ReviewVersion{}}}
	if path == "" {
		return s, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("快照损坏: %w", err)
	}
	if s.data.Batches == nil {
		s.data.Batches = map[string]model.InspectionBatch{}
	}
	if s.data.Observations == nil {
		s.data.Observations = map[string]model.Observation{}
	}
	if s.data.Tasks == nil {
		s.data.Tasks = map[string]model.RemediationTask{}
	}
	if s.data.Reviews == nil {
		s.data.Reviews = map[string][]model.ReviewVersion{}
	}
	return s, nil
}
func (s *Store) Snapshot() model.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.Marshal(s.data)
	var c model.Snapshot
	_ = json.Unmarshal(b, &c)
	return c
}
func (s *Store) SaveSnapshot(d model.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = d
	if s.path == "" {
		return nil
	}
	return s.persistLocked()
}
func (s *Store) persistLocked() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err = os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
func (s *Store) Update(fn func(*model.Snapshot) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(&s.data); err != nil {
		return err
	}
	if s.path != "" {
		return s.persistLocked()
	}
	return nil
}
func (s *Store) GetBatch(id string) (model.InspectionBatch, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.data.Batches[id]
	return b, ok
}
func (s *Store) ListBatches() []model.InspectionBatch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.InspectionBatch, 0, len(s.data.Batches))
	for _, b := range s.data.Batches {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}
func (s *Store) Observations(batchID string) []model.Observation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.Observation{}
	for _, o := range s.data.Observations {
		if o.BatchID == batchID {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RecordedAt.Before(out[j].RecordedAt) })
	return out
}
func (s *Store) Tasks(batchID string) []model.RemediationTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.RemediationTask{}
	for _, t := range s.data.Tasks {
		if t.BatchID == batchID {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DueAt.Before(out[j].DueAt) })
	return out
}

var ErrConflict = errors.New("revision conflict")
