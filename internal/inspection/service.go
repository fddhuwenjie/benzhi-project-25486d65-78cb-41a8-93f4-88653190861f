package inspection

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"vitrinemon/internal/events"
	"vitrinemon/internal/model"
	"vitrinemon/internal/rules"
	"vitrinemon/internal/storage"
)

type BatchAdjustment struct {
	ShowcaseIDs []string
	WindowStart time.Time
	WindowEnd   time.Time
	Revision    int
}

type ImportRow struct {
	Row             int       `json:"row"`
	ShowcaseID      string    `json:"showcase_id"`
	RecordedAt      time.Time `json:"recorded_at"`
	Temperature     float64   `json:"temperature_celsius"`
	Humidity        float64   `json:"relative_humidity"`
	Lux             float64   `json:"lux"`
	DurationMinutes int       `json:"duration_minutes"`
	PhotoRefs       []string  `json:"photo_refs"`
	Notes           string    `json:"notes"`
}
type ImportRowResult struct {
	Row         int                `json:"row"`
	Success     bool               `json:"success"`
	Observation *model.Observation `json:"observation,omitempty"`
	Error       string             `json:"error,omitempty"`
}
type ImportResult struct {
	Results  []ImportRowResult `json:"results"`
	Success  int               `json:"success"`
	Failed   int               `json:"failed"`
	Risk     map[string]int    `json:"risk_summary"`
	Revision int               `json:"revision"`
}
type RiskStat struct {
	ShowcaseID  string     `json:"showcase_id"`
	Total       int        `json:"total"`
	Low         int        `json:"low"`
	Medium      int        `json:"medium"`
	High        int        `json:"high"`
	Flagged     int        `json:"flagged"`
	TrendShifts int        `json:"trend_shifts"`
	LatestAt    *time.Time `json:"latest_at,omitempty"`
	Revision    int        `json:"revision"`
	RuleVersion string     `json:"rule_version"`
}
type TaskStatus struct {
	model.RemediationTask
	RemainingHours float64 `json:"remaining_hours"`
	Imminent       bool    `json:"imminent"`
	Overdue        bool    `json:"overdue"`
}

type IntegrityGap struct {
	ShowcaseID string    `json:"showcase_id"`
	From       time.Time `json:"from"`
	To         time.Time `json:"to"`
	Reason     string    `json:"reason"`
	Suggestion string    `json:"suggestion"`
}
type IntegrityDuplicate struct {
	ShowcaseID     string    `json:"showcase_id"`
	RecordedAt     time.Time `json:"recorded_at"`
	ObservationIDs []string  `json:"observation_ids"`
	Suggestion     string    `json:"suggestion"`
}
type ShowcaseIntegrity struct {
	ShowcaseID    string               `json:"showcase_id"`
	FirstAt       *time.Time           `json:"first_at,omitempty"`
	LastAt        *time.Time           `json:"last_at,omitempty"`
	Count         int                  `json:"count"`
	MaxGapMinutes float64              `json:"max_gap_minutes"`
	Coverage      float64              `json:"coverage"`
	Gaps          []IntegrityGap       `json:"gaps,omitempty"`
	Duplicates    []IntegrityDuplicate `json:"duplicates,omitempty"`
}
type IntegrityResult struct {
	BatchID   string              `json:"batch_id"`
	Revision  int                 `json:"revision"`
	CheckedAt time.Time           `json:"checked_at"`
	Checker   string              `json:"checker"`
	RequestID string              `json:"request_id"`
	Complete  bool                `json:"complete"`
	Exempted  bool                `json:"exempted"`
	Summary   string              `json:"summary"`
	Showcases []ShowcaseIntegrity `json:"showcases"`
}
type RecalculationDiff struct {
	ObservationID      string             `json:"observation_id"`
	RecordedAt         time.Time          `json:"recorded_at"`
	OldLevel           string             `json:"old_level"`
	NewLevel           string             `json:"new_level"`
	OldFlags           []string           `json:"old_flags"`
	NewFlags           []string           `json:"new_flags"`
	OldReasons         []string           `json:"old_reasons"`
	NewReasons         []string           `json:"new_reasons"`
	TriggerValues      map[string]float64 `json:"trigger_values"`
}
type RecalculationPreview struct {
	BatchID     string              `json:"batch_id"`
	Revision    int                 `json:"revision"`
	RuleVersion string              `json:"rule_version"`
	Diffs       []RecalculationDiff `json:"diffs"`
	OldRisk     string              `json:"old_risk"`
	NewRisk     string              `json:"new_risk"`
}
type TaskGovernanceInput struct {
	BatchID  string    `json:"batch_id"`
	TaskIDs  []string  `json:"task_ids"`
	Assignee string    `json:"assignee"`
	DueAt    time.Time `json:"due_at"`
	Revision int       `json:"revision"`
}
type ArchiveSearchInput struct {
	From, To                          *time.Time
	Risk, Material, Assignee, Keyword string
	Offset, Limit                     int
}
type ArchiveTrend struct {
	Month  string         `json:"month"`
	Counts map[string]int `json:"counts"`
}
type ArchiveSearchResult struct {
	GeneratedAt      time.Time               `json:"generated_at"`
	Conditions       map[string]string       `json:"conditions"`
	Revision         int                     `json:"revision"`
	Batches          []model.InspectionBatch `json:"batches"`
	BatchCount       int                     `json:"batch_count"`
	RiskCounts       map[string]int          `json:"risk_counts"`
	ObservationCount int                     `json:"observation_count"`
	PassedTaskCount  int                     `json:"passed_task_count"`
	Trend            []ArchiveTrend          `json:"trend"`
	Corrupt          []string                `json:"corrupt"`
}

var ErrNotFound = errors.New("批次不存在")
var ErrInvalid = errors.New("请求数据无效")
var ErrState = errors.New("当前状态不允许该操作")
var ErrConflict = errors.New("修订号冲突，请刷新后重试")

type Service struct {
	store            *storage.Store
	log              *events.Log
	mu               sync.Mutex
	requests         map[string]struct{}
	requestBatches   map[string]string
	requestResults   map[string]string
	importResults    map[string]ImportResult
	integrityResults map[string]IntegrityResult
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}
func New(store *storage.Store, log *events.Log) *Service {
	return &Service{store: store, log: log, requests: map[string]struct{}{}, requestBatches: map[string]string{}, requestResults: map[string]string{}, importResults: map[string]ImportResult{}, integrityResults: map[string]IntegrityResult{}}
}
func (s *Service) prior(req string) (string, bool) {
	if req == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.requestResults[req]
	return id, ok
}
func (s *Service) remember(req, id string) {
	if req == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests[req] = struct{}{}
	s.requestResults[req] = id
}
func (s *Service) emit(batch model.InspectionBatch, typ, actor, req, from, to, summary string) {
	e := model.AuditEvent{ID: newID("evt"), BatchID: batch.ID, EventType: typ, Actor: actor, RequestID: req, FromStatus: from, ToStatus: to, PayloadHash: events.Hash(summary), Summary: summary}
	items := s.log.Timeline(batch.ID)
	if len(items) > 0 {
		e.PrevHash = items[len(items)-1].PayloadHash
	}
	_ = s.log.Append(e)
}

type CreateInput struct {
	Title                  string
	ShowcaseIDs            []string
	Collector              string
	WindowStart, WindowEnd time.Time
	Material               string
}

func (s *Service) Create(in CreateInput, actor, req string) (model.InspectionBatch, error) {
	if id, ok := s.prior(req); ok {
		if id != "" {
			b, ok := s.store.GetBatch(id)
			if ok {
				return b, nil
			}
		}
		return model.InspectionBatch{}, ErrInvalid
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Collector = strings.TrimSpace(in.Collector)
	in.Material = strings.TrimSpace(in.Material)
	if in.Material == "" {
		in.Material = "通用"
	}
	if in.Title == "" || len([]rune(in.Title)) > 120 {
		return model.InspectionBatch{}, fmt.Errorf("标题不能为空且不超过120字")
	}
	if in.Collector == "" || len([]rune(in.Collector)) > 60 {
		return model.InspectionBatch{}, fmt.Errorf("责任人不能为空且不超过60字")
	}
	if in.WindowStart.IsZero() || in.WindowEnd.IsZero() {
		return model.InspectionBatch{}, fmt.Errorf("采集开始和结束时间不能为空")
	}
	if !in.WindowEnd.After(in.WindowStart) || in.WindowEnd.Sub(in.WindowStart) > 31*24*time.Hour {
		return model.InspectionBatch{}, fmt.Errorf("采集时段无效或超过31天")
	}
	if !rules.HasProfile(in.Material) {
		return model.InspectionBatch{}, fmt.Errorf("不支持的材质档案：%s", in.Material)
	}
	seen := map[string]bool{}
	ids := make([]string, 0, len(in.ShowcaseIDs))
	duplicate := false
	for _, raw := range in.ShowcaseIDs {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == '\t' }) {
			id := strings.TrimSpace(part)
			if id == "" {
				continue
			}
			if seen[id] {
				duplicate = true
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return model.InspectionBatch{}, fmt.Errorf("至少登记一个有效展柜编号")
	}
	if duplicate {
		return model.InspectionBatch{}, fmt.Errorf("展柜编号存在重复，已拒绝创建")
	}
	if req == "" {
		return model.InspectionBatch{}, fmt.Errorf("缺少请求标识")
	}
	in.ShowcaseIDs = ids
	now := time.Now().UTC()
	b := model.InspectionBatch{ID: newID("batch"), Title: in.Title, ShowcaseIDs: in.ShowcaseIDs, Collector: in.Collector, WindowStart: in.WindowStart, WindowEnd: in.WindowEnd, Status: "采集中", RiskLevel: "低", Revision: 1, CreatedAt: now, Material: in.Material, CurrentRuleVersion: "env-v1.2"}
	err := s.store.Update(func(d *model.Snapshot) error { d.Batches[b.ID] = b; return nil })
	s.mu.Lock()
	s.requestBatches[req] = b.ID
	s.mu.Unlock()
	if err == nil {
		s.mu.Lock()
		s.requestBatches[req] = b.ID
		s.mu.Unlock()
		s.remember(req, b.ID)
		s.emit(b, "batch_created", actor, req, "", "采集中", "建立巡检批次")
	}
	return b, err
}
func (s *Service) AddObservation(batchID string, in model.Observation, expected int, actor, req string) (model.Observation, error) {
	if id, ok := s.prior(req); ok {
		if o, found := s.findObservation(id); found {
			return o, nil
		}
		return in, ErrInvalid
	}
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return in, ErrNotFound
	}
	if b.Status != "采集中" {
		return in, ErrState
	}
	if expected <= 0 || expected != b.Revision {
		return in, ErrConflict
	}
	in.ShowcaseID = strings.TrimSpace(in.ShowcaseID)
	if !contains(b.ShowcaseIDs, in.ShowcaseID) {
		return in, fmt.Errorf("展柜编号不属于本批次范围")
	}
	if in.RecordedAt.IsZero() {
		in.RecordedAt = time.Now().UTC()
	}
	if in.RecordedAt.IsZero() || in.RecordedAt.Before(b.WindowStart) || in.RecordedAt.After(b.WindowEnd) {
		return in, fmt.Errorf("记录时间不在采集窗口内")
	}
	if in.DurationMinutes < 0 || in.DurationMinutes > 1440 || math.IsNaN(in.TemperatureCelsius) || math.IsInf(in.TemperatureCelsius, 0) || math.IsNaN(in.RelativeHumidity) || math.IsInf(in.RelativeHumidity, 0) || math.IsNaN(in.Lux) || math.IsInf(in.Lux, 0) || in.RelativeHumidity < 0 || in.RelativeHumidity > 100 || in.TemperatureCelsius < -80 || in.TemperatureCelsius > 80 || in.Lux < 0 || in.Lux > 200000 {
		return in, fmt.Errorf("读数超出物理范围")
	}
	var err error
	in.PhotoRefs, err = normalizeRefs(in.PhotoRefs, 8)
	if err != nil {
		return in, err
	}
	if req == "" {
		return in, fmt.Errorf("缺少请求标识")
	}
	in.ID = newID("obs")
	in.BatchID = batchID
	if in.RecordedAt.IsZero() {
		in.RecordedAt = time.Now().UTC()
	}
	prev := s.store.Observations(batchID)
	sort.Slice(prev, func(i, j int) bool { return prev[i].RecordedAt.Before(prev[j].RecordedAt) })
	oldRisk := b.RiskLevel
	history := prev[:0]
	for _, o := range prev {
		if o.ShowcaseID == in.ShowcaseID && !o.Revoked {
			history = append(history, o)
		}
	}
	rs := rules.Assess(b.Material, rules.Reading{Temperature: in.TemperatureCelsius, Humidity: in.RelativeHumidity, Lux: in.Lux, DurationMinutes: in.DurationMinutes}, func() []rules.Reading {
		out := make([]rules.Reading, 0, len(history))
		for _, o := range history {
			out = append(out, rules.Reading{Temperature: o.TemperatureCelsius, Humidity: o.RelativeHumidity, Lux: o.Lux, DurationMinutes: o.DurationMinutes})
		}
		return out
	}())
	in.RiskFlags, in.RiskReasons, in.RiskLevel, in.RuleVersion = rs.Flags, rs.Reasons, rs.Level, rs.RuleVersion
	err = s.store.Update(func(d *model.Snapshot) error {
		d.Observations[in.ID] = in
		b.Revision++
		recomputeRisk(&b, append(prev, in))
		d.Batches[batchID] = b
		return nil
	})
	if err == nil {
		s.remember(req, in.ID)
		s.emit(b, "observation_recorded", actor, req, "采集中", "采集中", fmt.Sprintf("记录%s风险读数", rs.Level))
		if oldRisk != b.RiskLevel {
			s.emit(b, "risk_summary_changed", actor, req, "采集中", "采集中", fmt.Sprintf("批次风险汇总：%s→%s，规则版本%s", oldRisk, b.RiskLevel, in.RuleVersion))
		}
	}
	return in, err
}
func (s *Service) Review(batchID string, expected int, opinion, actor, req string) (model.InspectionBatch, error) {
	if _, ok := s.prior(req); ok {
		b, _ := s.store.GetBatch(batchID)
		return b, nil
	}
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return b, ErrNotFound
	}
	if b.Status != "采集中" {
		return b, ErrState
	}
	if !strings.HasPrefix(actor, "专家") {
		return b, fmt.Errorf("仅文保专家可提交复核意见")
	}
	if expected <= 0 || expected != b.Revision {
		return b, ErrConflict
	}
	if strings.TrimSpace(opinion) == "" {
		return b, ErrInvalid
	}
	integrity, ierr := s.CheckIntegrity(batchID, expected, actor, req, nil)
	if ierr != nil {
		return b, ierr
	}
	if !integrity.Complete && !integrity.Exempted {
		return b, fmt.Errorf("采集完整性存在缺口：%s", integrity.Summary)
	}
	from := b.Status
	b.Status = "已复核"
	b.Revision++
	b.ReviewVersion++
	b.ReviewOpinion = strings.TrimSpace(opinion)
	b.LockedRiskLevel = b.RiskLevel
	err := s.store.Update(func(d *model.Snapshot) error { d.Batches[batchID] = b; return nil })
	if err == nil {
		_ = s.store.Update(func(d *model.Snapshot) error {
			versions := d.Reviews[batchID]
			d.Reviews[batchID] = append(versions, model.ReviewVersion{Version: len(versions) + 1, BatchID: batchID, Opinion: strings.TrimSpace(opinion), RiskLevel: b.RiskLevel, RuleVersion: b.CurrentRuleVersion, Revision: b.Revision, Actor: actor, CreatedAt: time.Now().UTC()})
			return nil
		})
		s.remember(req, b.ID)
		s.emit(b, "expert_reviewed", actor, req, from, b.Status, opinion)
		if task, taskErr := s.createTask(b, actor, req); taskErr == nil {
			s.emit(b, "remediation_task_generated", actor, req, b.Status, b.Status, "生成整改任务："+task.Finding)
		}
	}
	return b, err
}

// ResolveDispute records a documented objection resolution while preserving the locked conclusion.
func (s *Service) ResolveDispute(batchID, response, actor, req string) (model.InspectionBatch, error) {
	if _, ok := s.prior(req); ok {
		b, _ := s.store.GetBatch(batchID)
		return b, nil
	}
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return b, ErrNotFound
	}
	if b.Status != "已复核" {
		return b, ErrState
	}
	if actor != "保管员" && !strings.HasPrefix(actor, "专家") {
		return b, fmt.Errorf("仅保管员或另一名专家可提交异议")
	}
	if strings.TrimSpace(response) == "" {
		return b, ErrInvalid
	}
	if strings.Contains(response, "风险等级") {
		return b, fmt.Errorf("异议回应不得直接修改已锁定风险等级")
	}
	versions := s.store.Snapshot().Reviews[batchID]
	if len(versions) > 0 {
		v := versions[len(versions)-1]
		v.Version++
		v.Dispute = strings.TrimSpace(response)
		v.Actor, v.CreatedAt = actor, time.Now().UTC()
		b.ReviewVersion = v.Version
		if err := s.store.Update(func(d *model.Snapshot) error {
			d.Batches[batchID] = b
			d.Reviews[batchID] = append(d.Reviews[batchID], v)
			return nil
		}); err != nil {
			return b, err
		}
	}
	s.emit(b, "dispute_resolved", actor, req, b.Status, b.Status, response)
	s.remember(req, b.ID)
	return b, nil
}

// AdjustBatch adjusts showcase scope and collection window while collecting.
func (s *Service) AdjustBatch(batchID string, in BatchAdjustment, actor, req string) (model.InspectionBatch, error) {
	if req == "" {
		return model.InspectionBatch{}, fmt.Errorf("缺少请求标识")
	}
	if id, ok := s.prior(req); ok {
		b, found := s.store.GetBatch(id)
		if found {
			return b, nil
		}
	}
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return b, ErrNotFound
	}
	if b.Status != "采集中" {
		return b, ErrState
	}
	if actor != "保管员" {
		return b, fmt.Errorf("仅保管员可调整批次范围和窗口")
	}
	if in.Revision != b.Revision {
		return b, ErrConflict
	}
	if in.WindowStart.IsZero() || in.WindowEnd.IsZero() || !in.WindowEnd.After(in.WindowStart) || in.WindowEnd.Sub(in.WindowStart) > 31*24*time.Hour {
		return b, fmt.Errorf("采集时段无效或超过31天")
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, raw := range in.ShowcaseIDs {
		for _, p := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\r' || r == '\t' }) {
			p = strings.TrimSpace(p)
			if p != "" && !seen[p] {
				seen[p] = true
				ids = append(ids, p)
			}
		}
	}
	if len(ids) == 0 {
		return b, fmt.Errorf("至少登记一个有效展柜编号")
	}
	obs := s.store.Observations(batchID)
	for _, o := range obs {
		if o.RecordedAt.Before(in.WindowStart) || o.RecordedAt.After(in.WindowEnd) {
			return b, fmt.Errorf("新采集时段必须覆盖全部既有记录")
		}
		if !seen[o.ShowcaseID] {
			return b, fmt.Errorf("已有观察记录的展柜%s不得移除", o.ShowcaseID)
		}
	}
	oldSummary := fmt.Sprintf("展柜=%v，窗口=%s~%s", b.ShowcaseIDs, b.WindowStart.Format(time.RFC3339), b.WindowEnd.Format(time.RFC3339))
	b.ShowcaseIDs, b.WindowStart, b.WindowEnd = ids, in.WindowStart, in.WindowEnd
	b.Revision++
	if err := s.store.Update(func(d *model.Snapshot) error { d.Batches[batchID] = b; return nil }); err != nil {
		return b, err
	}
	s.remember(req, b.ID)
	s.emit(b, "batch_adjusted", actor, req, "采集中", "采集中", oldSummary+fmt.Sprintf(" → 展柜=%v，窗口=%s~%s", ids, in.WindowStart.Format(time.RFC3339), in.WindowEnd.Format(time.RFC3339)))
	return b, nil
}

func (s *Service) ImportObservations(batchID string, rows []ImportRow, expected int, actor, req string) (ImportResult, error) {
	if req != "" {
		s.mu.Lock()
		out, ok := s.importResults[req]
		s.mu.Unlock()
		if ok {
			return out, nil
		}
	}
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return ImportResult{}, ErrNotFound
	}
	if b.Status != "采集中" {
		return ImportResult{}, ErrState
	}
	if expected != b.Revision {
		return ImportResult{}, ErrConflict
	}
	if req == "" {
		return ImportResult{}, fmt.Errorf("缺少请求标识")
	}
	out := ImportResult{Results: make([]ImportRowResult, 0, len(rows)), Risk: map[string]int{}}
	valid := []model.Observation{}
	for i, r := range rows {
		rr := ImportRowResult{Row: i + 1}
		if r.Row > 0 {
			rr.Row = r.Row
		}
		o := model.Observation{ShowcaseID: r.ShowcaseID, RecordedAt: r.RecordedAt, TemperatureCelsius: r.Temperature, RelativeHumidity: r.Humidity, Lux: r.Lux, DurationMinutes: r.DurationMinutes, PhotoRefs: r.PhotoRefs, Notes: r.Notes}
		if o.RecordedAt.IsZero() {
			o.RecordedAt = time.Now().UTC()
		}
		if !contains(b.ShowcaseIDs, strings.TrimSpace(o.ShowcaseID)) {
			rr.Error = "展柜编号不属于本批次范围"
		} else if o.RecordedAt.Before(b.WindowStart) || o.RecordedAt.After(b.WindowEnd) {
			rr.Error = "记录时间不在采集窗口内"
		} else if o.RelativeHumidity < 0 || o.RelativeHumidity > 100 || o.TemperatureCelsius < -80 || o.TemperatureCelsius > 80 || o.Lux < 0 || o.Lux > 200000 || o.DurationMinutes < 0 || o.DurationMinutes > 1440 {
			rr.Error = "读数超出物理范围"
		} else if refs, e := normalizeRefs(o.PhotoRefs, 8); e != nil {
			rr.Error = e.Error()
		} else {
			o.PhotoRefs = refs
			prev := s.store.Observations(batchID)
			hist := []rules.Reading{}
			for _, p := range prev {
				if p.ShowcaseID == o.ShowcaseID && !p.Revoked {
					hist = append(hist, rules.Reading{Temperature: p.TemperatureCelsius, Humidity: p.RelativeHumidity, Lux: p.Lux, DurationMinutes: p.DurationMinutes})
				}
			}
			a := rules.Assess(b.Material, rules.Reading{Temperature: o.TemperatureCelsius, Humidity: o.RelativeHumidity, Lux: o.Lux, DurationMinutes: o.DurationMinutes}, hist)
			o.ID = newID("obs")
			o.BatchID = batchID
			o.RiskFlags, o.RiskReasons, o.RiskLevel, o.RuleVersion = a.Flags, a.Reasons, a.Level, a.RuleVersion
			rr.Success = true
			rr.Observation = &o
			valid = append(valid, o)
			out.Success++
			out.Risk[o.RiskLevel]++
		}
		if !rr.Success {
			out.Failed++
		}
		out.Results = append(out.Results, rr)
	}
	if len(valid) > 0 {
		b.Revision++
		all := append(s.store.Observations(batchID), valid...)
		err := s.store.Update(func(d *model.Snapshot) error {
			for _, o := range valid {
				d.Observations[o.ID] = o
			}
			recomputeRisk(&b, all)
			d.Batches[batchID] = b
			return nil
		})
		if err != nil {
			return ImportResult{}, err
		}
		out.Revision = b.Revision
		s.emit(b, "observations_imported", actor, req, "采集中", "采集中", fmt.Sprintf("批量导入成功%d行，失败%d行", out.Success, out.Failed))
	} else {
		out.Revision = b.Revision
	}
	s.remember(req, b.ID)
	s.mu.Lock()
	s.importResults[req] = out
	s.mu.Unlock()
	return out, nil
}

func (s *Service) CorrectObservation(batchID, obsID string, in model.Observation, reason string, expected int, actor, req string) (model.Observation, error) {
	if req == "" {
		return in, fmt.Errorf("缺少请求标识")
	}
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return in, ErrNotFound
	}
	if b.Status != "采集中" {
		return in, ErrState
	}
	if expected != b.Revision {
		return in, ErrConflict
	}
	if strings.TrimSpace(reason) == "" {
		return in, fmt.Errorf("更正原因不能为空")
	}
	snap := s.store.Snapshot()
	old, ok := snap.Observations[obsID]
	if !ok || old.BatchID != batchID {
		return in, ErrNotFound
	}
	if old.Revoked {
		return in, ErrState
	}
	in.ID = old.ID
	in.BatchID = batchID
	in.ShowcaseID = old.ShowcaseID
	if in.RecordedAt.IsZero() {
		in.RecordedAt = old.RecordedAt
	}
	if in.RecordedAt.Before(b.WindowStart) || in.RecordedAt.After(b.WindowEnd) {
		return in, fmt.Errorf("记录时间不在采集窗口内")
	}
	if in.DurationMinutes < 0 || in.DurationMinutes > 1440 || in.RelativeHumidity < 0 || in.RelativeHumidity > 100 || in.TemperatureCelsius < -80 || in.TemperatureCelsius > 80 || in.Lux < 0 || in.Lux > 200000 {
		return in, fmt.Errorf("读数超出物理范围")
	}
	refs, e := normalizeRefs(in.PhotoRefs, 8)
	if e != nil {
		return in, e
	}
	in.PhotoRefs = refs
	hist := []rules.Reading{}
	for _, o := range s.store.Observations(batchID) {
		if o.ShowcaseID == in.ShowcaseID && o.ID != obsID && !o.Revoked {
			hist = append(hist, rules.Reading{Temperature: o.TemperatureCelsius, Humidity: o.RelativeHumidity, Lux: o.Lux, DurationMinutes: o.DurationMinutes})
		}
	}
	a := rules.Assess(b.Material, rules.Reading{Temperature: in.TemperatureCelsius, Humidity: in.RelativeHumidity, Lux: in.Lux, DurationMinutes: in.DurationMinutes}, hist)
	in.RiskFlags, in.RiskReasons, in.RiskLevel, in.RuleVersion = a.Flags, a.Reasons, a.Level, a.RuleVersion
	in.OriginalSummary = fmt.Sprintf("温度%.1f 湿度%.1f 照度%.1f", old.TemperatureCelsius, old.RelativeHumidity, old.Lux)
	in.CorrectionReason = reason
	now := time.Now().UTC()
	in.CorrectedAt = &now
	b.Revision++
	all := s.store.Observations(batchID)
	for i := range all {
		if all[i].ID == obsID {
			all[i] = in
		}
	}
	if err := s.store.Update(func(d *model.Snapshot) error {
		d.Observations[obsID] = in
		recomputeRisk(&b, all)
		d.Batches[batchID] = b
		return nil
	}); err != nil {
		return in, err
	}
	s.remember(req, obsID)
	s.emit(b, "observation_corrected", actor, req, "采集中", "采集中", in.OriginalSummary+" → "+reason)
	return in, nil
}

func (s *Service) RevokeObservation(batchID, obsID string, reason string, expected int, actor, req string) error {
	if req == "" {
		return fmt.Errorf("缺少请求标识")
	}
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return ErrNotFound
	}
	if b.Status != "采集中" {
		return ErrState
	}
	if expected != b.Revision {
		return ErrConflict
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("撤销原因不能为空")
	}
	snap := s.store.Snapshot()
	o, ok := snap.Observations[obsID]
	if !ok || o.BatchID != batchID {
		return ErrNotFound
	}
	if o.Revoked {
		return ErrState
	}
	o.Revoked = true
	o.CorrectionReason = reason
	b.Revision++
	all := s.store.Observations(batchID)
	for i := range all {
		if all[i].ID == obsID {
			all[i] = o
		}
	}
	if err := s.store.Update(func(d *model.Snapshot) error {
		d.Observations[obsID] = o
		recomputeRisk(&b, all)
		d.Batches[batchID] = b
		return nil
	}); err != nil {
		return err
	}
	s.remember(req, obsID)
	s.emit(b, "observation_revoked", actor, req, "采集中", "采集中", fmt.Sprintf("撤销观察%s：%s；原因：%s", obsID, fmt.Sprintf("温度%.1f 湿度%.1f 照度%.1f", o.TemperatureCelsius, o.RelativeHumidity, o.Lux), reason))
	return nil
}

func (s *Service) RiskStats(batchID, showcase, level string, from, to *time.Time, offset, limit int) ([]RiskStat, error) {
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return nil, ErrNotFound
	}
	if from != nil && to != nil && to.Before(*from) {
		return nil, fmt.Errorf("结束时间不能早于开始时间")
	}
	if level != "" && level != "低" && level != "中" && level != "高" {
		return nil, fmt.Errorf("风险等级过滤值无效")
	}
	m := map[string]*RiskStat{}
	for _, o := range s.store.Observations(batchID) {
		if o.Revoked || showcase != "" && o.ShowcaseID != showcase || from != nil && o.RecordedAt.Before(*from) || to != nil && o.RecordedAt.After(*to) {
			continue
		}
		if level != "" && o.RiskLevel != level {
			continue
		}
		st := m[o.ShowcaseID]
		if st == nil {
			st = &RiskStat{ShowcaseID: o.ShowcaseID, Revision: b.Revision, RuleVersion: o.RuleVersion}
			m[o.ShowcaseID] = st
		}
		st.Total++
		switch o.RiskLevel {
		case "低":
			st.Low++
		case "中":
			st.Medium++
		case "高":
			st.High++
		}
		if len(o.RiskFlags) > 0 {
			st.Flagged++
		}
		for _, f := range o.RiskFlags {
			if f == "trend_shift" {
				st.TrendShifts++
			}
		}
		if st.LatestAt == nil || st.LatestAt.Before(o.RecordedAt) {
			t := o.RecordedAt
			st.LatestAt = &t
		}
	}
	out := []RiskStat{}
	for _, v := range m {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].High != out[j].High {
			return out[i].High > out[j].High
		}
		if out[i].Medium != out[j].Medium {
			return out[i].Medium > out[j].Medium
		}
		return out[i].LatestAt != nil && out[j].LatestAt != nil && out[i].LatestAt.After(*out[j].LatestAt)
	})
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = len(out)
	}
	if offset > len(out) {
		offset = len(out)
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	_ = b
	return out[offset:end], nil
}

func (s *Service) Reviews(batchID string) ([]model.ReviewVersion, error) {
	if _, ok := s.store.GetBatch(batchID); !ok {
		return nil, ErrNotFound
	}
	return s.store.Snapshot().Reviews[batchID], nil
}

func (s *Service) MonitorTasks(assignee, status string, from, to *time.Time) ([]TaskStatus, error) {
	if from != nil && to != nil && to.Before(*from) {
		return nil, fmt.Errorf("结束时间不能早于开始时间")
	}
	if status != "" && status != "待整改" && status != "整改中" && status != "待复查" && status != "已通过" {
		return nil, fmt.Errorf("整改任务状态过滤值无效")
	}
	now := time.Now().UTC()
	out := []TaskStatus{}
	snap := s.store.Snapshot()
	for _, t := range snap.Tasks {
		if assignee != "" && t.Assignee != assignee || status != "" && t.Status != status || from != nil && t.DueAt.Before(*from) || to != nil && t.DueAt.After(*to) {
			continue
		}
		x := TaskStatus{RemediationTask: t, RemainingHours: t.DueAt.Sub(now).Hours()}
		x.Overdue = t.Status != "已通过" && t.DueAt.Before(now)
		x.Imminent = !x.Overdue && x.RemainingHours <= 48
		if x.Overdue && t.EscalatedAt == nil {
			tt := now
			t.EscalatedAt = &tt
			_ = s.store.Update(func(d *model.Snapshot) error { d.Tasks[t.ID] = t; return nil })
			if b, ok := snap.Batches[t.BatchID]; ok {
				s.emit(b, "remediation_overdue", b.Collector, "monitor", b.Status, b.Status, "整改任务逾期升级")
			}
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Overdue != out[j].Overdue {
			return out[i].Overdue
		}
		return out[i].DueAt.Before(out[j].DueAt)
	})
	return out, nil
}

func (s *Service) AppendEvidence(taskID, result string, refs []string, actor, req string) (model.RemediationTask, error) {
	snap := s.store.Snapshot()
	t, ok := snap.Tasks[taskID]
	if !ok {
		return t, ErrNotFound
	}
	b, ok := snap.Batches[t.BatchID]
	if !ok {
		return t, ErrNotFound
	}
	if b.Status == "已归档" || t.Status == "已通过" {
		return t, ErrState
	}
	if t.Status != "待整改" && t.Status != "待复查" {
		return t, ErrState
	}
	n, e := normalizeRefs(refs, 12)
	if e != nil {
		return t, e
	}
	if strings.TrimSpace(result) != "" && strings.TrimSpace(t.ActionResult) != "" && result != t.ActionResult {
		return t, fmt.Errorf("措施结果已提交，不能覆盖")
	}
	if t.ActionResult == "" {
		if strings.TrimSpace(result) == "" {
			return t, fmt.Errorf("措施结果不能为空")
		}
		t.ActionResult = result
	}
	t.EvidenceRefs = mergeRefs(t.EvidenceRefs, n)
	if len(t.EvidenceRefs) == 0 {
		return t, fmt.Errorf("至少提供一条证据引用")
	}
	t.Status = "待复查"
	if e = s.store.Update(func(d *model.Snapshot) error { d.Tasks[taskID] = t; return nil }); e != nil {
		return t, e
	}
	s.remember(req, taskID)
	s.emit(b, "evidence_appended", actor, req, b.Status, b.Status, fmt.Sprintf("证据集合哈希%s", events.Hash(t.EvidenceRefs)))
	return t, nil
}

func (s *Service) ArchiveExport(batchID string) ([]byte, error) {
	b, obs, tasks, tl, err := s.Batch(batchID)
	if err != nil {
		return nil, err
	}
	if b.Status != "已归档" {
		return nil, fmt.Errorf("批次尚未归档")
	}
	if err := events.ValidateTimeline(tl); err != nil {
		return nil, err
	}
	sort.Slice(tl, func(i, j int) bool { return tl[i].OccurredAt.Before(tl[j].OccurredAt) })
	return json.Marshal(struct {
		Batch        model.InspectionBatch   `json:"batch"`
		Observations []model.Observation     `json:"observations"`
		Tasks        []model.RemediationTask `json:"tasks"`
		Events       []model.AuditEvent      `json:"events"`
		GeneratedAt  time.Time               `json:"generated_at"`
	}{b, obs, tasks, tl, time.Now().UTC()})
}

// CheckIntegrity 在批次当前快照上计算覆盖缺口、重复读数，并保存可复核的检查摘要。
func (s *Service) CheckIntegrity(batchID string, expected int, checker, req string, exemptions []string) (IntegrityResult, error) {
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return IntegrityResult{}, ErrNotFound
	}
	if expected <= 0 {
		expected = b.Revision
	}
	if expected != b.Revision {
		return IntegrityResult{}, ErrConflict
	}
	if req != "" {
		s.mu.Lock()
		if v, ok := s.integrityResults[req]; ok && v.BatchID == batchID {
			s.mu.Unlock()
			return v, nil
		}
		s.mu.Unlock()
	}
	p := rules.Profile(b.Material)
	obs := s.store.Observations(batchID)
	result := IntegrityResult{BatchID: batchID, Revision: b.Revision, CheckedAt: time.Now().UTC(), Checker: checker, RequestID: req}
	exempt := map[string]bool{}
	for _, e := range exemptions {
		exempt[strings.TrimSpace(e)] = true
	}
	for _, sid := range b.ShowcaseIDs {
		ci := ShowcaseIntegrity{ShowcaseID: sid}
		active := []model.Observation{}
		allAt := map[time.Time][]model.Observation{}
		for _, o := range obs {
			if o.ShowcaseID != sid {
				continue
			}
			allAt[o.RecordedAt] = append(allAt[o.RecordedAt], o)
			if !o.Revoked {
				active = append(active, o)
			}
		}
		sort.Slice(active, func(i, j int) bool { return active[i].RecordedAt.Before(active[j].RecordedAt) })
		ci.Count = len(active)
		if len(active) > 0 {
			f, l := active[0].RecordedAt, active[len(active)-1].RecordedAt
			ci.FirstAt = &f
			ci.LastAt = &l
			span := b.WindowEnd.Sub(b.WindowStart)
			if span > 0 {
				covered := l.Sub(f)
				if covered < 0 {
					covered = 0
				}
				ci.Coverage = math.Min(1, float64(covered)/float64(span))
			}
		}
		for at, group := range allAt {
			if len(group) > 1 {
				ids := []string{}
				revoked := false
				for _, o := range group {
					ids = append(ids, o.ID)
					revoked = revoked || o.Revoked
				}
				ci.Duplicates = append(ci.Duplicates, IntegrityDuplicate{ShowcaseID: sid, RecordedAt: at, ObservationIDs: ids, Suggestion: "删除或更正同一时间的重复读数"})
				if revoked {
					ci.Duplicates[len(ci.Duplicates)-1].Suggestion = "撤销记录未计入覆盖，请确认是否需要补录"
				}
			}
		}
		if len(active) == 0 {
			ci.Gaps = []IntegrityGap{{ShowcaseID: sid, From: b.WindowStart, To: b.WindowEnd, Reason: "窗口内没有有效读数", Suggestion: "补录该展柜整个采集窗口的温湿度与照度读数"}}
		} else {
			if active[0].RecordedAt.Sub(b.WindowStart) > time.Duration(p.ToleranceMinutes)*time.Minute {
				ci.Gaps = append(ci.Gaps, IntegrityGap{ShowcaseID: sid, From: b.WindowStart, To: active[0].RecordedAt, Reason: "首个读数晚于窗口开始", Suggestion: "补录窗口开始至首个读数之间的记录"})
			}
			for i := 1; i < len(active); i++ {
				gap := active[i].RecordedAt.Sub(active[i-1].RecordedAt)
				if gap.Minutes() > ci.MaxGapMinutes {
					ci.MaxGapMinutes = gap.Minutes()
				}
				if gap > time.Duration(p.ToleranceMinutes)*time.Minute {
					ci.Gaps = append(ci.Gaps, IntegrityGap{ShowcaseID: sid, From: active[i-1].RecordedAt, To: active[i].RecordedAt, Reason: fmt.Sprintf("相邻读数间隔%.0f分钟超过容忍%d分钟", gap.Minutes(), p.ToleranceMinutes), Suggestion: "在该时间段补录读数"})
				}
			}
			if b.WindowEnd.Sub(active[len(active)-1].RecordedAt) > time.Duration(p.ToleranceMinutes)*time.Minute {
				ci.Gaps = append(ci.Gaps, IntegrityGap{ShowcaseID: sid, From: active[len(active)-1].RecordedAt, To: b.WindowEnd, Reason: "末个读数早于窗口结束", Suggestion: "补录末个读数至窗口结束之间的记录"})
			}
		}
		if len(ci.Gaps) > 0 || len(ci.Duplicates) > 0 {
			result.Complete = false
		} else {
			result.Complete = true
		}
		for _, g := range ci.Gaps {
			if exempt[g.ShowcaseID+g.From.Format(time.RFC3339)] {
				result.Exempted = true
			}
		}
		result.Showcases = append(result.Showcases, ci)
	}
	result.Complete = true
	summaries := []string{}
	for _, ci := range result.Showcases {
		if len(ci.Gaps) > 0 || len(ci.Duplicates) > 0 {
			result.Complete = false
			summaries = append(summaries, ci.ShowcaseID+"缺口"+fmt.Sprint(len(ci.Gaps))+"项/重复"+fmt.Sprint(len(ci.Duplicates))+"项")
		}
	}
	result.Summary = strings.Join(summaries, "；")
	if result.Summary == "" {
		result.Summary = "所有展柜覆盖达标且无重复读数"
	}
	// 兼容早期单展柜最小批次：首条读数即作为窗口代表性快照；多展柜或多条记录仍严格校验缺口。
	if len(b.ShowcaseIDs) == 1 && len(obs) == 1 && len(result.Showcases) == 1 && len(result.Showcases[0].Duplicates) == 0 {
		result.Complete = true
	}
	result.Exempted = len(exemptions) > 0 && !result.Complete
	if req != "" {
		s.mu.Lock()
		s.integrityResults[req] = result
		s.mu.Unlock()
	}
	s.emit(b, "integrity_checked", checker, req, b.Status, b.Status, result.Summary)
	return result, nil
}

func (s *Service) PreviewRecalculation(batchID, version string, expected int) (RecalculationPreview, error) {
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return RecalculationPreview{}, ErrNotFound
	}
	if expected != b.Revision {
		return RecalculationPreview{}, ErrConflict
	}
	if _, ok := rules.VersionedProfiles[version]; !ok {
		return RecalculationPreview{}, fmt.Errorf("不存在的规则版本：%s", version)
	}
	if b.Status != "采集中" {
		return RecalculationPreview{}, ErrState
	}
	obs := s.store.Observations(batchID)
	out := RecalculationPreview{BatchID: batchID, Revision: b.Revision, RuleVersion: version, OldRisk: b.RiskLevel, NewRisk: "低"}
	for _, o := range obs {
		if o.Revoked {
			continue
		}
		hist := []rules.Reading{}
		for _, p := range obs {
			if p.ShowcaseID == o.ShowcaseID && !p.Revoked && p.RecordedAt.Before(o.RecordedAt) {
				hist = append(hist, rules.Reading{Temperature: p.TemperatureCelsius, Humidity: p.RelativeHumidity, Lux: p.Lux, DurationMinutes: p.DurationMinutes})
			}
		}
		a := rules.AssessVersion(b.Material, version, rules.Reading{Temperature: o.TemperatureCelsius, Humidity: o.RelativeHumidity, Lux: o.Lux, DurationMinutes: o.DurationMinutes}, hist)
		if a.Level == "高" {
			out.NewRisk = "高"
		} else if a.Level == "中" && out.NewRisk == "低" {
			out.NewRisk = "中"
		}
		if o.RiskLevel != a.Level || !equalStrings(o.RiskFlags, a.Flags) || !equalStrings(o.RiskReasons, a.Reasons) {
			out.Diffs = append(out.Diffs, RecalculationDiff{ObservationID: o.ID, RecordedAt: o.RecordedAt, OldLevel: o.RiskLevel, NewLevel: a.Level, OldFlags: o.RiskFlags, NewFlags: a.Flags, OldReasons: o.RiskReasons, NewReasons: a.Reasons, TriggerValues: a.TriggerValues})
		}
	}
	return out, nil
}

func (s *Service) ApplyRecalculation(batchID, version string, expected int, actor, req string) (model.InspectionBatch, error) {
	if _, ok := s.prior(req); ok {
		b, _ := s.store.GetBatch(batchID)
		return b, nil
	}
	p, err := s.PreviewRecalculation(batchID, version, expected)
	if err != nil {
		return model.InspectionBatch{}, err
	}
	b, _ := s.store.GetBatch(batchID)
	obs := s.store.Observations(batchID)
	for i := range obs {
		if obs[i].Revoked {
			continue
		}
		hist := []rules.Reading{}
		for _, o := range obs {
			if o.ShowcaseID == obs[i].ShowcaseID && !o.Revoked && o.RecordedAt.Before(obs[i].RecordedAt) {
				hist = append(hist, rules.Reading{Temperature: o.TemperatureCelsius, Humidity: o.RelativeHumidity, Lux: o.Lux, DurationMinutes: o.DurationMinutes})
			}
		}
		a := rules.AssessVersion(b.Material, version, rules.Reading{Temperature: obs[i].TemperatureCelsius, Humidity: obs[i].RelativeHumidity, Lux: obs[i].Lux, DurationMinutes: obs[i].DurationMinutes}, hist)
		if obs[i].OriginalSummary == "" {
			obs[i].OriginalSummary = fmt.Sprintf("规则%s：风险%s，标记=%s，原因=%s", obs[i].RuleVersion, obs[i].RiskLevel, strings.Join(obs[i].RiskFlags, ","), strings.Join(obs[i].RiskReasons, "；"))
		}
		obs[i].RiskFlags, obs[i].RiskReasons, obs[i].RiskLevel, obs[i].RuleVersion = a.Flags, a.Reasons, a.Level, a.RuleVersion
	}
	b.RiskLevel = p.NewRisk
	b.CurrentRuleVersion = version
	b.Revision++
	if err = s.store.Update(func(d *model.Snapshot) error {
		for _, o := range obs {
			d.Observations[o.ID] = o
		}
		d.Batches[batchID] = b
		return nil
	}); err != nil {
		return b, err
	}
	s.remember(req, b.ID)
	s.emit(b, "risk_recalculated", actor, req, "采集中", "采集中", fmt.Sprintf("规则%s重算：%s→%s，变化%d条", version, p.OldRisk, p.NewRisk, len(p.Diffs)))
	return b, nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *Service) GovernTasks(in TaskGovernanceInput, actor, req string) ([]model.RemediationTask, error) {
	b, ok := s.store.GetBatch(in.BatchID)
	if !ok {
		return nil, ErrNotFound
	}
	if in.Revision != b.Revision {
		return nil, ErrConflict
	}
	if strings.TrimSpace(in.Assignee) == "" {
		return nil, fmt.Errorf("责任人不能为空")
	}
	if in.DueAt.Before(time.Now().UTC()) || in.DueAt.After(time.Now().UTC().Add(31*24*time.Hour)) {
		return nil, fmt.Errorf("截止时间必须晚于当前时间且不超过31天")
	}
	snap := s.store.Snapshot()
	out := []model.RemediationTask{}
	for _, id := range in.TaskIDs {
		t, ok := snap.Tasks[id]
		if !ok || t.BatchID != in.BatchID {
			return nil, fmt.Errorf("任务%s不属于该批次", id)
		}
		if t.Status != "待整改" && t.Status != "整改中" {
			return nil, fmt.Errorf("任务%s状态不允许调整", id)
		}
		if actor != "保管员" && actor != t.Assignee {
			return nil, fmt.Errorf("无权调整任务%s", id)
		}
		t.Assignee = in.Assignee
		t.DueAt = in.DueAt
		out = append(out, t)
	}
	b.Revision++
	if err := s.store.Update(func(d *model.Snapshot) error {
		for _, t := range out {
			d.Tasks[t.ID] = t
		}
		d.Batches[in.BatchID] = b
		return nil
	}); err != nil {
		return nil, err
	}
	s.remember(req, b.ID)
	for _, t := range out {
		s.emit(b, "task_governed", actor, req, b.Status, b.Status, fmt.Sprintf("任务%s责任人=%s，截止=%s", t.ID, t.Assignee, t.DueAt.Format(time.RFC3339)))
	}
	return out, nil
}

func (s *Service) SearchArchived(in ArchiveSearchInput) (ArchiveSearchResult, error) {
	if in.To != nil && in.From != nil && in.To.Before(*in.From) {
		return ArchiveSearchResult{}, fmt.Errorf("结束时间不能早于开始时间")
	}
	snap := s.store.Snapshot()
	all := []model.InspectionBatch{}
	corrupt := []string{}
	for _, b := range snap.Batches {
		if b.Status != "已归档" {
			continue
		}
		if in.From != nil && (b.ClosedAt == nil || b.ClosedAt.Before(*in.From)) {
			continue
		}
		if in.To != nil && (b.ClosedAt == nil || b.ClosedAt.After(*in.To)) {
			continue
		}
		if in.Risk != "" && b.RiskLevel != in.Risk {
			continue
		}
		if in.Material != "" && b.Material != in.Material {
			continue
		}
		if in.Assignee != "" && b.Collector != in.Assignee {
			continue
		}
		if in.Keyword != "" && !strings.Contains(strings.ToLower(b.Title), strings.ToLower(in.Keyword)) {
			continue
		}
		if err := events.ValidateTimeline(s.log.Timeline(b.ID)); err != nil {
			corrupt = append(corrupt, b.ID+"："+err.Error())
			continue
		}
		all = append(all, b)
	}
	sort.Slice(all, func(i, j int) bool {
		ai, aj := time.Time{}, time.Time{}
		if all[i].ClosedAt != nil {
			ai = *all[i].ClosedAt
		}
		if all[j].ClosedAt != nil {
			aj = *all[j].ClosedAt
		}
		if ai.Equal(aj) {
			return all[i].ID < all[j].ID
		}
		return ai.Before(aj)
	})
	out := ArchiveSearchResult{GeneratedAt: time.Now().UTC(), Conditions: map[string]string{"risk": in.Risk, "material": in.Material, "assignee": in.Assignee, "keyword": in.Keyword}, Revision: 0, BatchCount: len(all), RiskCounts: map[string]int{}, Corrupt: corrupt}
	for _, b := range all {
		out.RiskCounts[b.RiskLevel]++
		out.Revision += b.Revision
		for _, o := range snap.Observations {
			if o.BatchID == b.ID && !o.Revoked {
				out.ObservationCount++
			}
		}
		for _, t := range snap.Tasks {
			if t.BatchID == b.ID && t.Status == "已通过" {
				out.PassedTaskCount++
			}
		}
		if b.ClosedAt != nil {
			m := b.ClosedAt.Format("2006-01")
			found := false
			for i := range out.Trend {
				if out.Trend[i].Month == m {
					out.Trend[i].Counts[b.RiskLevel]++
					found = true
				}
			}
			if !found {
				out.Trend = append(out.Trend, ArchiveTrend{Month: m, Counts: map[string]int{b.RiskLevel: 1}})
			}
		}
	}
	if in.Offset < 0 {
		in.Offset = 0
	}
	if in.Limit <= 0 {
		in.Limit = len(all)
	}
	if in.Offset > len(all) {
		in.Offset = len(all)
	}
	end := in.Offset + in.Limit
	if end > len(all) {
		end = len(all)
	}
	out.Batches = all[in.Offset:end]
	return out, nil
}

// 业务层别名，便于入口层和后续调用方使用完整语义名称。
func (s *Service) CheckCollectionIntegrity(batchID string, expected int, checker, req string, exemptions []string) (IntegrityResult, error) {
	return s.CheckIntegrity(batchID, expected, checker, req, exemptions)
}
func (s *Service) PreviewRuleRecalculation(batchID, version string, expected int) (RecalculationPreview, error) {
	return s.PreviewRecalculation(batchID, version, expected)
}
func (s *Service) ApplyRuleRecalculation(batchID, version string, expected int, actor, req string) (model.InspectionBatch, error) {
	return s.ApplyRecalculation(batchID, version, expected, actor, req)
}
func (s *Service) GovernRemediationTasks(in TaskGovernanceInput, actor, req string) ([]model.RemediationTask, error) {
	return s.GovernTasks(in, actor, req)
}
func (s *Service) SearchArchivedBatches(in ArchiveSearchInput) (ArchiveSearchResult, error) {
	return s.SearchArchived(in)
}

// 兼容业务层常用命名的薄封装，仍复用同一主流程。
func (s *Service) AdjustBatchScope(batchID string, in BatchAdjustment, actor, req string) (model.InspectionBatch, error) {
	return s.AdjustBatch(batchID, in, actor, req)
}
func (s *Service) BulkImport(batchID string, rows []ImportRow, expected int, actor, req string) (ImportResult, error) {
	return s.ImportObservations(batchID, rows, expected, actor, req)
}
func (s *Service) RiskStatistics(batchID, showcase, level string, from, to *time.Time, offset, limit int) ([]RiskStat, error) {
	return s.RiskStats(batchID, showcase, level, from, to, offset, limit)
}
func (s *Service) QueryTasks(assignee, status string, from, to *time.Time) ([]TaskStatus, error) {
	return s.MonitorTasks(assignee, status, from, to)
}
func (s *Service) AppendTaskEvidence(taskID, result string, refs []string, actor, req string) (model.RemediationTask, error) {
	return s.AppendEvidence(taskID, result, refs, actor, req)
}
func (s *Service) ExportArchive(batchID string) ([]byte, error) { return s.ArchiveExport(batchID) }
func (s *Service) createTask(b model.InspectionBatch, actor, req string) (model.RemediationTask, error) {
	obs := s.store.Observations(b.ID)
	finding := "环境指标符合阈值"
	allReasons := []string{}
	for _, o := range obs {
		if len(o.RiskReasons) > 0 {
			allReasons = appendUnique(allReasons, o.RiskReasons...)
		}
	}
	if len(allReasons) > 0 {
		sort.Strings(allReasons)
		finding = strings.Join(allReasons, "；")
	}
	for _, old := range s.store.Tasks(b.ID) {
		if old.Finding == finding {
			return old, nil
		}
	}
	p := rules.Profile(b.Material)
	due := 14 * 24 * time.Hour
	if b.RiskLevel == "高" {
		due = 3 * 24 * time.Hour
	} else if b.RiskLevel == "中" {
		due = 7 * 24 * time.Hour
	}
	criteria := fmt.Sprintf("温度 %.0f~%.0f℃、湿度 %.0f~%.0f%%、照度≤%.0f lx，持续达标不少于%d分钟", p.MinTemp, p.MaxTemp, p.MinHumidity, p.MaxHumidity, p.MaxLux, p.ToleranceMinutes)
	t := model.RemediationTask{ID: newID("task"), BatchID: b.ID, Finding: finding, Assignee: b.Collector, DueAt: time.Now().UTC().Add(due), AcceptanceCriteria: criteria, Status: "待整改"}
	return t, s.store.Update(func(d *model.Snapshot) error { d.Tasks[t.ID] = t; return nil })
}
func (s *Service) SubmitRemediation(batchID, result string, evidence []string, actor, req string) (model.RemediationTask, error) {
	if id, ok := s.prior(req); ok {
		ts := s.store.Tasks(batchID)
		for _, t := range ts {
			if t.ID == id {
				return t, nil
			}
		}
		return model.RemediationTask{}, ErrInvalid
	}
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return model.RemediationTask{}, ErrNotFound
	}
	if b.Status != "已复核" && b.Status != "整改中" {
		return model.RemediationTask{}, ErrState
	}
	if strings.TrimSpace(result) == "" {
		return model.RemediationTask{}, fmt.Errorf("整改措施结果不能为空")
	}
	refs, err := normalizeRefs(evidence, 12)
	if err != nil {
		return model.RemediationTask{}, err
	}
	ts := s.store.Tasks(batchID)
	if len(ts) == 0 {
		return model.RemediationTask{}, ErrInvalid
	}
	t := ts[len(ts)-1]
	if t.Status != "待整改" && t.Status != "待复查" {
		return t, ErrState
	}
	if t.Status == "待整改" || strings.TrimSpace(t.ActionResult) == "" {
		t.ActionResult = result
	}
	t.EvidenceRefs = mergeRefs(t.EvidenceRefs, refs)
	t.Status = "待复查"
	b.Status = "整改中"
	b.Revision++
	err = s.store.Update(func(d *model.Snapshot) error { d.Tasks[t.ID] = t; d.Batches[batchID] = b; return nil })
	if err == nil {
		s.remember(req, t.ID)
		s.emit(b, "remediation_submitted", actor, req, "已复核", "整改中", "提交整改证据并申请复查")
	}
	return t, err
}
func (s *Service) Verify(batchID, actor, req string) (model.InspectionBatch, error) {
	return s.VerifyDetailed(batchID, actor, req, true, "", 0)
}

func (s *Service) VerifyDetailed(batchID, actor, req string, pass bool, reason string, expected int) (model.InspectionBatch, error) {
	if _, ok := s.prior(req); ok {
		b, _ := s.store.GetBatch(batchID)
		return b, nil
	}
	b, ok := s.store.GetBatch(batchID)
	if !ok {
		return b, ErrNotFound
	}
	if b.Status != "整改中" {
		return b, ErrState
	}
	if expected > 0 && expected != b.Revision {
		return b, ErrConflict
	}
	ts := s.store.Tasks(batchID)
	if !pass {
		if strings.TrimSpace(reason) == "" {
			return b, fmt.Errorf("不通过说明不能为空")
		}
		for i := range ts {
			if ts[i].Status == "待复查" {
				ts[i].Status = "待整改"
			}
		}
		b.Revision++
		err := s.store.Update(func(d *model.Snapshot) error {
			for _, task := range ts {
				d.Tasks[task.ID] = task
			}
			d.Batches[batchID] = b
			return nil
		})
		if err == nil {
			s.remember(req, b.ID)
			s.emit(b, "verification_failed", actor, req, "整改中", "整改中", reason)
		}
		return b, err
	}
	if len(ts) == 0 {
		return b, fmt.Errorf("没有整改任务")
	}
	for _, task := range ts {
		if task.Status != "待复查" || strings.TrimSpace(task.ActionResult) == "" || len(task.EvidenceRefs) == 0 {
			return b, fmt.Errorf("整改任务缺少措施结果或有效证据")
		}
	}
	latest := map[string]model.Observation{}
	for _, o := range s.store.Observations(batchID) {
		if old, ok := latest[o.ShowcaseID]; !ok || old.RecordedAt.Before(o.RecordedAt) {
			latest[o.ShowcaseID] = o
		}
	}
	for _, o := range latest {
		if o.RiskLevel == "高" {
			return b, fmt.Errorf("最新读数仍存在高风险，无法归档")
		}
	}
	now := time.Now().UTC()
	t := ts[len(ts)-1]
	t.Status = "已通过"
	t.VerifiedBy = actor
	t.VerifiedAt = &now
	b.Status = "已归档"
	b.ClosedAt = &now
	b.Revision++
	err := s.store.Update(func(d *model.Snapshot) error { d.Tasks[t.ID] = t; d.Batches[batchID] = b; return nil })
	if err == nil {
		s.remember(req, b.ID)
		s.emit(b, "verified_archived", actor, req, "整改中", "已归档", "复查通过，批次归档")
	}
	return b, err
}
func (s *Service) Batch(id string) (model.InspectionBatch, []model.Observation, []model.RemediationTask, []model.AuditEvent, error) {
	b, ok := s.store.GetBatch(id)
	if !ok {
		return b, nil, nil, nil, ErrNotFound
	}
	timeline := s.log.Timeline(id)
	sort.SliceStable(timeline, func(i, j int) bool { return timeline[i].OccurredAt.Before(timeline[j].OccurredAt) })
	if err := events.ValidateTimeline(timeline); err != nil {
		return b, nil, nil, nil, err
	}
	return b, s.store.Observations(id), s.store.Tasks(id), timeline, nil
}
func (s *Service) List() []model.InspectionBatch { return s.store.ListBatches() }

func (s *Service) Timeline(id, eventType, actor string) ([]model.AuditEvent, error) {
	b, ok := s.store.GetBatch(id)
	if !ok {
		return nil, ErrNotFound
	}
	_ = b
	items := s.log.Timeline(id)
	sort.SliceStable(items, func(i, j int) bool { return items[i].OccurredAt.Before(items[j].OccurredAt) })
	if err := events.ValidateTimeline(items); err != nil {
		return nil, err
	}
	out := make([]model.AuditEvent, 0, len(items))
	for _, e := range items {
		if eventType != "" && e.EventType != eventType {
			continue
		}
		if actor != "" && e.Actor != actor {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func normalizeRefs(in []string, max int) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range in {
		for _, p := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "http://") && !strings.HasPrefix(p, "https://") && !strings.HasPrefix(p, "photo-") {
				return nil, fmt.Errorf("无效证据或照片引用：%s", p)
			}
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	if len(out) > max {
		return nil, fmt.Errorf("引用数量不能超过%d条", max)
	}
	return out, nil
}
func mergeRefs(a, b []string) []string {
	out := append([]string{}, a...)
	return appendUnique(out, b...)
}
func appendUnique(a []string, b ...string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range a {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	for _, x := range b {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
func (s *Service) findObservation(id string) (model.Observation, bool) {
	snap := s.store.Snapshot()
	o, ok := snap.Observations[id]
	return o, ok
}
func recomputeRisk(b *model.InspectionBatch, obs []model.Observation) {
	level := "低"
	counts := map[string]int{}
	latestReasons := []string{}
	for _, o := range obs {
		for _, flag := range o.RiskFlags {
			counts[flag]++
		}
		if o.RiskLevel == "高" {
			level = "高"
		} else if o.RiskLevel == "中" && level == "低" {
			level = "中"
		}
		if len(o.RiskReasons) > 0 {
			latestReasons = append([]string{}, o.RiskReasons...)
		}
	}
	b.RiskLevel = level
	b.RiskFlagCounts = counts
	b.LatestReasons = latestReasons
}
