package inspection

import (
	"testing"
	"time"
	"vitrinemon/internal/events"
	"vitrinemon/internal/model"
	"vitrinemon/internal/storage"
)

func TestLifecycle(t *testing.T) {
	st, _ := storage.New("")
	svc := New(st, events.New(""))
	now := time.Now()
	b, err := svc.Create(CreateInput{Title: "测试", ShowcaseIDs: []string{"A"}, Collector: "保管员", WindowStart: now, WindowEnd: now.Add(time.Hour)}, "保管员", "c1")
	if err != nil {
		t.Fatal(err)
	}
	o, err := svc.AddObservation(b.ID, model.Observation{ShowcaseID: "A", TemperatureCelsius: 20, RelativeHumidity: 50, Lux: 20, DurationMinutes: 10}, b.Revision, "保管员", "o1")
	if err != nil || o.ID == "" {
		t.Fatal(err)
	}
	b, err = svc.Review(b.ID, b.Revision+1, "同意", "专家", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SubmitRemediation(b.ID, "已调整", []string{"photo-1"}, "负责人", "m1"); err != nil {
		t.Fatal(err)
	}
	b, err = svc.Verify(b.ID, "保管员", "v1")
	if err != nil || b.Status != "已归档" {
		t.Fatalf("归档失败: %v %#v", err, b)
	}
}
