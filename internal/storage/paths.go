package storage

import "path/filepath"

func SnapshotFilename(dir string) string {
	if dir == "" {
		return "snapshot.json"
	}
	return filepath.Join(dir, "snapshot.json")
}
