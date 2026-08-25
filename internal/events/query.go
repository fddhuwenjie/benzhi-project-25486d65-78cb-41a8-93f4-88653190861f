package events

import "vitrinemon/internal/model"

func Filter(items []model.AuditEvent, eventType string) []model.AuditEvent {
	if eventType == "" {
		return append([]model.AuditEvent(nil), items...)
	}
	out := make([]model.AuditEvent, 0, len(items))
	for _, item := range items {
		if item.EventType == eventType {
			out = append(out, item)
		}
	}
	return out
}
