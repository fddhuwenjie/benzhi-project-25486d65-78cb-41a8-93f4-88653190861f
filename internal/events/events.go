package events

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
	"vitrinemon/internal/model"
)

type Log struct {
	mu     sync.Mutex
	path   string
	memory []model.AuditEvent
}

func New(path string) *Log {
	l := &Log{path: path}
	if path == "" {
		return l
	}
	f, err := os.Open(path)
	if err != nil {
		return l
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e model.AuditEvent
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			l.memory = append(l.memory, e)
		}
	}
	return l
}
func Hash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (l *Log) Append(e model.AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.ID == "" {
		e.ID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	l.memory = append(l.memory, e)
	if l.path == "" {
		return nil
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(e)
}
func (l *Log) Timeline(batchID string) []model.AuditEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := []model.AuditEvent{}
	for _, e := range l.memory {
		if e.BatchID == batchID {
			out = append(out, e)
		}
	}
	return out
}

// ValidateTimeline 检查相邻事件的状态是否连续，供归档页面和运维诊断使用。
func ValidateTimeline(items []model.AuditEvent) error {
	if len(items) == 0 {
		return fmt.Errorf("审计时间线为空")
	}
	for i := 1; i < len(items); i++ {
		if items[i].FromStatus != items[i-1].ToStatus {
			return fmt.Errorf("审计时间线在事件 %s 处状态不连续", items[i].ID)
		}
		if items[i].PayloadHash == "" {
			return fmt.Errorf("审计事件 %s 缺少摘要哈希", items[i].ID)
		}
		if items[i].PayloadHash != Hash(items[i].Summary) {
			return fmt.Errorf("审计事件 %s 摘要哈希不匹配", items[i].ID)
		}
		if items[i].PrevHash != "" && items[i].PrevHash != items[i-1].PayloadHash {
			return fmt.Errorf("审计事件 %s 前序摘要哈希不匹配", items[i].ID)
		}
	}
	if items[0].PayloadHash == "" {
		return fmt.Errorf("审计事件 %s 缺少摘要哈希", items[0].ID)
	}
	if items[0].PayloadHash != Hash(items[0].Summary) {
		return fmt.Errorf("审计事件 %s 摘要哈希不匹配", items[0].ID)
	}
	return nil
}
