package storage

import "os"

func (s *Store) Path() string { return s.path }

func (s *Store) HasData() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data.Batches) > 0 || len(s.data.Observations) > 0 || len(s.data.Tasks) > 0
}

func (s *Store) ExistsOnDisk() bool {
	if s.path == "" {
		return false
	}
	_, err := os.Stat(s.path)
	return err == nil
}
