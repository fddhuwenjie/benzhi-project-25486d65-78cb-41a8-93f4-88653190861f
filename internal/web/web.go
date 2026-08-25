package web

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"vitrinemon/internal/inspection"
	"vitrinemon/internal/model"
)

type Server struct {
	app       *inspection.Service
	mux       *http.ServeMux
	templates *template.Template
}

func New(app *inspection.Service) *Server {
	s := &Server{app: app, mux: http.NewServeMux()}
	s.routes()
	return s
}
func (s *Server) Handler() http.Handler { return s.mux }
func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.health)
	s.mux.HandleFunc("/", s.home)
	s.mux.HandleFunc("/inspection", s.inspection)
	s.mux.HandleFunc("/inspection/", s.detail)
	s.mux.HandleFunc("/api/batches", s.apiBatches)
	s.mux.HandleFunc("/api/observations", s.apiObservation)
	s.mux.HandleFunc("/api/observations/import", s.apiObservationImport)
	s.mux.HandleFunc("/api/observations/correct", s.apiObservationCorrect)
	s.mux.HandleFunc("/api/observations/revoke", s.apiObservationRevoke)
	s.mux.HandleFunc("/api/batches/adjust", s.apiBatchAdjust)
	s.mux.HandleFunc("/api/risk-stats", s.apiRiskStats)
	s.mux.HandleFunc("/api/tasks", s.apiTasks)
	s.mux.HandleFunc("/api/evidence", s.apiEvidence)
	s.mux.HandleFunc("/api/archive/export", s.apiArchiveExport)
	s.mux.HandleFunc("/api/reviews", s.apiReviews)
	s.mux.HandleFunc("/api/review", s.apiReview)
	s.mux.HandleFunc("/api/dispute", s.apiDispute)
	s.mux.HandleFunc("/api/remediation", s.apiRemediation)
	s.mux.HandleFunc("/api/integrity", s.apiIntegrity)
	s.mux.HandleFunc("/api/collection-integrity", s.apiIntegrity)
	s.mux.HandleFunc("/api/rules/preview", s.apiRulesPreview)
	s.mux.HandleFunc("/api/rules/apply", s.apiRulesApply)
	s.mux.HandleFunc("/api/rule-recalculation/preview", s.apiRulesPreview)
	s.mux.HandleFunc("/api/rule-recalculation/apply", s.apiRulesApply)
	s.mux.HandleFunc("/api/tasks/govern", s.apiTasksGovern)
	s.mux.HandleFunc("/api/archive/search", s.apiArchiveSearch)
	s.mux.HandleFunc("/api/archives/search", s.apiArchiveSearch)
	s.mux.HandleFunc("/api/verify", s.apiVerify)
	s.mux.HandleFunc("/api/timeline", s.apiTimeline)
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))
}

func (s *Server) apiIntegrity(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("batch_id")
	rev, _ := strconv.Atoi(r.URL.Query().Get("revision"))
	if id == "" {
		http.Error(w, "缺少 batch_id", 400)
		return
	}
	var ex []string
	if v := r.URL.Query().Get("exemptions"); v != "" {
		ex = strings.Split(v, ",")
	}
	out, e := s.app.CheckIntegrity(id, rev, actor(r), reqID(r), ex)
	if e != nil {
		http.Error(w, e.Error(), 409)
		return
	}
	writeJSON(w, out)
}
func (s *Server) apiRulesPreview(w http.ResponseWriter, r *http.Request) {
	var p struct {
		BatchID, RuleVersion string
		Revision             int `json:"revision"`
	}
	if !decode(w, r, &p) {
		return
	}
	out, e := s.app.PreviewRecalculation(p.BatchID, p.RuleVersion, p.Revision)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	writeJSON(w, out)
}
func (s *Server) apiRulesApply(w http.ResponseWriter, r *http.Request) {
	var p struct {
		BatchID, RuleVersion string
		Revision             int `json:"revision"`
	}
	if !decode(w, r, &p) {
		return
	}
	out, e := s.app.ApplyRecalculation(p.BatchID, p.RuleVersion, p.Revision, actor(r), reqID(r))
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	writeJSON(w, out)
}
func (s *Server) apiTasksGovern(w http.ResponseWriter, r *http.Request) {
	var p struct {
		BatchID  string   `json:"batch_id"`
		TaskIDs  []string `json:"task_ids"`
		Assignee string   `json:"assignee"`
		DueAt    string   `json:"due_at"`
		Revision int      `json:"revision"`
	}
	if !decode(w, r, &p) {
		return
	}
	due, e := time.Parse(time.RFC3339, p.DueAt)
	if e != nil {
		http.Error(w, "截止时间格式错误", 400)
		return
	}
	out, e := s.app.GovernTasks(inspection.TaskGovernanceInput{BatchID: p.BatchID, TaskIDs: p.TaskIDs, Assignee: p.Assignee, DueAt: due, Revision: p.Revision}, actor(r), reqID(r))
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	writeJSON(w, out)
}
func (s *Server) apiArchiveSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var from, to *time.Time
	var e error
	if v := q.Get("from"); v != "" {
		x, er := time.Parse(time.RFC3339, v)
		if er != nil {
			http.Error(w, "开始时间格式错误", 400)
			return
		}
		from = &x
	}
	if v := q.Get("to"); v != "" {
		x, er := time.Parse(time.RFC3339, v)
		if er != nil {
			http.Error(w, "结束时间格式错误", 400)
			return
		}
		to = &x
	}
	off, _ := strconv.Atoi(q.Get("offset"))
	lim, _ := strconv.Atoi(q.Get("limit"))
	out, e := s.app.SearchArchived(inspection.ArchiveSearchInput{From: from, To: to, Risk: q.Get("risk"), Material: q.Get("material"), Assignee: q.Get("assignee"), Keyword: q.Get("keyword"), Offset: off, Limit: lim})
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	writeJSON(w, out)
}

func (s *Server) apiBatchAdjust(w http.ResponseWriter, r *http.Request) {
	var p struct {
		BatchID     string   `json:"batch_id"`
		ShowcaseIDs []string `json:"showcase_ids"`
		WindowStart string   `json:"window_start"`
		WindowEnd   string   `json:"window_end"`
		Revision    int      `json:"revision"`
	}
	if !decode(w, r, &p) {
		return
	}
	st, e1 := time.Parse(time.RFC3339, p.WindowStart)
	en, e2 := time.Parse(time.RFC3339, p.WindowEnd)
	if e1 != nil || e2 != nil {
		http.Error(w, "采集窗口格式错误", 400)
		return
	}
	b, err := s.app.AdjustBatch(p.BatchID, inspection.BatchAdjustment{ShowcaseIDs: p.ShowcaseIDs, WindowStart: st, WindowEnd: en, Revision: p.Revision}, actor(r), reqID(r))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, b)
}

func (s *Server) apiObservationImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST", 405)
		return
	}
	if r.Header.Get("X-Request-ID") == "" {
		http.Error(w, "缺少 X-Request-ID 请求标识", 400)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var p struct {
		BatchID  string                 `json:"batch_id"`
		Revision int                    `json:"revision"`
		Rows     []inspection.ImportRow `json:"rows"`
	}
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "csv") {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "文件读取失败", 400)
			return
		}
		recs, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
		if err != nil {
			http.Error(w, "CSV格式错误", 400)
			return
		}
		for i, row := range recs {
			if i == 0 && len(row) > 0 && strings.Contains(strings.ToLower(row[0]), "showcase") {
				continue
			}
			if len(row) < 6 {
				continue
			}
			t, _ := time.Parse(time.RFC3339, row[1])
			var temp, hum, lux float64
			var dur int
			fmt.Sscanf(row[2], "%f", &temp)
			fmt.Sscanf(row[3], "%f", &hum)
			fmt.Sscanf(row[4], "%f", &lux)
			fmt.Sscanf(row[5], "%d", &dur)
			p.Rows = append(p.Rows, inspection.ImportRow{Row: i + 1, ShowcaseID: row[0], RecordedAt: t, Temperature: temp, Humidity: hum, Lux: lux, DurationMinutes: dur})
		}
	} else if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "JSON格式错误", 400)
		return
	}
	if p.BatchID == "" {
		p.BatchID = r.URL.Query().Get("batch_id")
	}
	out, err := s.app.ImportObservations(p.BatchID, p.Rows, p.Revision, actor(r), reqID(r))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, out)
}

func (s *Server) apiObservationCorrect(w http.ResponseWriter, r *http.Request) {
	var p struct {
		BatchID       string            `json:"batch_id"`
		ObservationID string            `json:"observation_id"`
		Reason        string            `json:"reason"`
		Revision      int               `json:"revision"`
		Observation   model.Observation `json:"observation"`
	}
	if !decode(w, r, &p) {
		return
	}
	if p.ObservationID == "" {
		p.ObservationID = p.Observation.ID
	}
	o, err := s.app.CorrectObservation(p.BatchID, p.ObservationID, p.Observation, p.Reason, p.Revision, actor(r), reqID(r))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, o)
}
func (s *Server) apiObservationRevoke(w http.ResponseWriter, r *http.Request) {
	var p struct {
		BatchID       string `json:"batch_id"`
		ObservationID string `json:"observation_id"`
		Reason        string `json:"reason"`
		Revision      int    `json:"revision"`
	}
	if !decode(w, r, &p) {
		return
	}
	if err := s.app.RevokeObservation(p.BatchID, p.ObservationID, p.Reason, p.Revision, actor(r), reqID(r)); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, map[string]string{"status": "已撤销"})
}
func (s *Server) apiRiskStats(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("batch_id")
	var f, t *time.Time
	var err error
	if v := r.URL.Query().Get("from"); v != "" {
		x, e := time.Parse(time.RFC3339, v)
		if e != nil {
			http.Error(w, "开始时间格式错误", 400)
			return
		}
		f = &x
	}
	if v := r.URL.Query().Get("to"); v != "" {
		x, e := time.Parse(time.RFC3339, v)
		if e != nil {
			http.Error(w, "结束时间格式错误", 400)
			return
		}
		t = &x
	}
	off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	lim, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := s.app.RiskStats(id, r.URL.Query().Get("showcase_id"), r.URL.Query().Get("level"), f, t, off, lim)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, out)
}
func (s *Server) apiTasks(w http.ResponseWriter, r *http.Request) {
	var f, t *time.Time
	var e error
	if v := r.URL.Query().Get("from"); v != "" {
		x, er := time.Parse(time.RFC3339, v)
		if er != nil {
			http.Error(w, "时间格式错误", 400)
			return
		}
		f = &x
	}
	if v := r.URL.Query().Get("to"); v != "" {
		x, er := time.Parse(time.RFC3339, v)
		if er != nil {
			http.Error(w, "时间格式错误", 400)
			return
		}
		t = &x
	}
	out, e := s.app.MonitorTasks(r.URL.Query().Get("assignee"), r.URL.Query().Get("status"), f, t)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	writeJSON(w, out)
}
func (s *Server) apiEvidence(w http.ResponseWriter, r *http.Request) {
	var p struct {
		TaskID, Result string
		Evidence       []string `json:"evidence_refs"`
	}
	if !decode(w, r, &p) {
		return
	}
	t, e := s.app.AppendEvidence(p.TaskID, p.Result, p.Evidence, actor(r), reqID(r))
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	writeJSON(w, t)
}
func (s *Server) apiArchiveExport(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("batch_id")
	if id == "" {
		http.Error(w, "缺少 batch_id", 400)
		return
	}
	b, e := s.app.ArchiveExport(id)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=archive.json")
	w.Write(b)
}
func (s *Server) apiReviews(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("batch_id")
	v, e := s.app.Reviews(id)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	writeJSON(w, v)
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"status": "ok", "service": "vitrinemon"})
}
func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/inspection", http.StatusFound)
}
func (s *Server) inspection(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.createForm(w, r)
		return
	}
	batches := s.app.List()
	data := struct{ Batches []model.InspectionBatch }{batches}
	render(w, "inspection.html", data)
}
func (s *Server) detail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/inspection/")
	if id == "" {
		http.Redirect(w, r, "/inspection", http.StatusFound)
		return
	}
	b, obs, tasks, events, err := s.app.Batch(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	render(w, "detail.html", struct {
		Batch        model.InspectionBatch
		Observations []model.Observation
		Tasks        []model.RemediationTask
		Events       []model.AuditEvent
	}{b, obs, tasks, events})
}
func render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, err := template.ParseFiles("web/" + name)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_ = t.Execute(w, data)
}
func actor(r *http.Request) string {
	a := r.Header.Get("X-Actor")
	if a == "" {
		a = "保管员"
	}
	return a
}
func reqID(r *http.Request) string {
	v := r.Header.Get("X-Request-ID")
	if v == "" {
		v = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return v
}
func (s *Server) createForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "表单格式错误", 400)
		return
	}
	start, _ := time.Parse("2006-01-02T15:04", r.FormValue("window_start"))
	end, _ := time.Parse("2006-01-02T15:04", r.FormValue("window_end"))
	show := strings.FieldsFunc(r.FormValue("showcase_ids"), func(c rune) bool { return c == ',' || c == ' ' || c == '\n' })
	b, err := s.app.Create(inspection.CreateInput{Title: r.FormValue("title"), ShowcaseIDs: show, Collector: r.FormValue("collector"), WindowStart: start, WindowEnd: end, Material: r.FormValue("material")}, actor(r), reqID(r))
	if err != nil {
		http.Error(w, "创建失败："+err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/inspection/"+b.ID, http.StatusSeeOther)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Header.Get("X-Request-ID") == "" {
		http.Error(w, "缺少 X-Request-ID 请求标识", 400)
		return false
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "JSON 格式错误", 400)
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

type batchPayload struct {
	Title       string   `json:"title"`
	ShowcaseIDs []string `json:"showcase_ids"`
	Collector   string   `json:"collector"`
	WindowStart string   `json:"window_start"`
	WindowEnd   string   `json:"window_end"`
	Material    string   `json:"material"`
}

func (s *Server) apiBatches(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, s.app.List())
		return
	}
	var p batchPayload
	if !decode(w, r, &p) {
		return
	}
	st, _ := time.Parse(time.RFC3339, p.WindowStart)
	en, _ := time.Parse(time.RFC3339, p.WindowEnd)
	b, err := s.app.Create(inspection.CreateInput{Title: p.Title, ShowcaseIDs: p.ShowcaseIDs, Collector: p.Collector, WindowStart: st, WindowEnd: en, Material: p.Material}, actor(r), reqID(r))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, b)
}

type obsPayload struct {
	BatchID     string   `json:"batch_id"`
	ShowcaseID  string   `json:"showcase_id"`
	Temperature float64  `json:"temperature_celsius"`
	Humidity    float64  `json:"relative_humidity"`
	Lux         float64  `json:"lux"`
	Duration    int      `json:"duration_minutes"`
	Notes       string   `json:"notes"`
	PhotoRefs   []string `json:"photo_refs"`
	Revision    int      `json:"revision"`
	RecordedAt  string   `json:"recorded_at"`
}

func (s *Server) apiObservation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST", 405)
		return
	}
	var p obsPayload
	if !decode(w, r, &p) {
		return
	}
	if strings.TrimSpace(p.RecordedAt) == "" {
		http.Error(w, "记录时间不能为空", 400)
		return
	}
	recorded := time.Time{}
	if p.RecordedAt != "" {
		recorded, _ = time.Parse(time.RFC3339, p.RecordedAt)
	}
	o, err := s.app.AddObservation(p.BatchID, model.Observation{ShowcaseID: p.ShowcaseID, RecordedAt: recorded, TemperatureCelsius: p.Temperature, RelativeHumidity: p.Humidity, Lux: p.Lux, DurationMinutes: p.Duration, Notes: p.Notes, PhotoRefs: p.PhotoRefs}, p.Revision, actor(r), reqID(r))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, o)
}

type reviewPayload struct {
	BatchID  string `json:"batch_id"`
	Opinion  string `json:"opinion"`
	Revision int    `json:"revision"`
}

func (s *Server) apiReview(w http.ResponseWriter, r *http.Request) {
	var p reviewPayload
	if !decode(w, r, &p) {
		return
	}
	b, err := s.app.Review(p.BatchID, p.Revision, p.Opinion, actor(r), reqID(r))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, b)
}

func (s *Server) apiDispute(w http.ResponseWriter, r *http.Request) {
	var p struct {
		BatchID  string `json:"batch_id"`
		Response string `json:"response"`
	}
	if !decode(w, r, &p) {
		return
	}
	b, err := s.app.ResolveDispute(p.BatchID, p.Response, actor(r), reqID(r))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, b)
}

type remediationPayload struct {
	BatchID  string   `json:"batch_id"`
	Result   string   `json:"action_result"`
	Evidence []string `json:"evidence_refs"`
}

func (s *Server) apiRemediation(w http.ResponseWriter, r *http.Request) {
	var p remediationPayload
	if !decode(w, r, &p) {
		return
	}
	t, err := s.app.SubmitRemediation(p.BatchID, p.Result, p.Evidence, actor(r), reqID(r))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, t)
}
func (s *Server) apiVerify(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("batch_id")
	var p struct {
		BatchID  string `json:"batch_id"`
		Pass     *bool  `json:"pass"`
		Reason   string `json:"reason"`
		Revision int    `json:"revision"`
	}
	if id == "" {
		if !decode(w, r, &p) {
			return
		}
		id = p.BatchID
	} else if r.Method == http.MethodPost && r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&p)
	}
	pass := true
	if p.Pass != nil {
		pass = *p.Pass
	}
	b, err := s.app.VerifyDetailed(id, actor(r), reqID(r), pass, p.Reason, p.Revision)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, b)
}

func (s *Server) apiTimeline(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("batch_id")
	if id == "" {
		http.Error(w, "缺少 batch_id", 400)
		return
	}
	items, err := s.app.Timeline(id, r.URL.Query().Get("event_type"), r.URL.Query().Get("actor"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if v := r.URL.Query().Get("from"); v != "" {
		if t, e := time.Parse(time.RFC3339, v); e == nil {
			filtered := items[:0]
			for _, item := range items {
				if !item.OccurredAt.Before(t) {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		} else {
			http.Error(w, "时间范围格式错误", 400)
			return
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, e := time.Parse(time.RFC3339, v); e == nil {
			filtered := items[:0]
			for _, item := range items {
				if !item.OccurredAt.After(t) {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		} else {
			http.Error(w, "时间范围格式错误", 400)
			return
		}
	}
	offset, limit := 0, len(items)
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, e := strconv.Atoi(v); e == nil && n >= 0 {
			offset = n
		} else {
			http.Error(w, "分页参数错误", 400)
			return
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, e := strconv.Atoi(v); e == nil && n > 0 {
			limit = n
		} else {
			http.Error(w, "分页参数错误", 400)
			return
		}
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	items = items[offset:end]
	writeJSON(w, items)
}
