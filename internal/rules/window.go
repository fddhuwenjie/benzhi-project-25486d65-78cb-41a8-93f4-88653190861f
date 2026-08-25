package rules

import (
	"strings"
	"time"
)

func InWindow(at, start, end time.Time) bool {
	return !at.Before(start) && !at.After(end)
}

func NormalizeVersion(version string) string {
	if strings.TrimSpace(version) == "" {
		return "env-v1.2"
	}
	if _, ok := VersionedProfiles[version]; ok {
		return version
	}
	return "env-v1.2"
}
