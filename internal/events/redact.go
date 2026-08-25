package events

import "strings"

func RedactActor(actor string) string {
	a := strings.TrimSpace(actor)
	if a == "" {
		return "匿名操作者"
	}
	r := []rune(a)
	if len(r) <= 1 {
		return "*"
	}
	return string(r[0]) + strings.Repeat("*", len(r)-1)
}
