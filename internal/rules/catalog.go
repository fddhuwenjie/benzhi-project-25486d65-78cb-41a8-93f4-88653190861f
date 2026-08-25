package rules

import "sort"

func Profiles() []MaterialProfile {
	out := make([]MaterialProfile, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
