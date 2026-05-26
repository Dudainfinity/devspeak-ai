package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ai é inicializado em main(); pode ser nil se OPENAI_API_KEY estiver vazia.
var ai *aiClient

// ──────────────────────────────────────────────────────────────────────────────
// Tipos compartilhados
// ──────────────────────────────────────────────────────────────────────────────

type Profile struct {
	Name            string `json:"name"`
	Stack           string `json:"stack"`           // backend | frontend | fullstack | devops | data | mobile
	Level           string `json:"level"`           // junior | mid | senior
	YearsExperience int    `json:"yearsExperience"`
	PrimaryLanguage string `json:"primaryLanguage"` // Go | Python | Java | JavaScript | TypeScript | Other
	TargetRole      string `json:"targetRole"`      // descrição da vaga / job posting
}

type Question struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Topic string `json:"topic"`
	Stack string `json:"stack"`
	Level string `json:"level"`
}

type questionsRequest struct {
	Profile Profile `json:"profile"`
	Limit   int     `json:"limit"`
}

type questionsResponse struct {
	Questions []Question `json:"questions"`
	Profile   Profile    `json:"profile"`
}

type evaluateRequest struct {
	Level    string   `json:"level"`
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	Profile  *Profile `json:"profile,omitempty"`
}

type evaluateResponse struct {
	Score      string   `json:"score"`
	ScoreClass string   `json:"scoreClass"`
	Technical  string   `json:"technical"`
	English    []string `json:"english"`
	Vocab      []string `json:"vocab"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Banco de perguntas (stack × nível)
// ──────────────────────────────────────────────────────────────────────────────

var questionBank = []Question{
	// backend – junior
	{"be-j-1", `Explain the difference between <em>concurrency</em> and <em>parallelism</em>. Give a real-world example of each.`, "backend", "backend", "junior"},
	{"be-j-2", `What is a <em>REST API</em>? What makes it RESTful?`, "backend", "backend", "junior"},
	{"be-j-3", `What is the difference between <em>SQL</em> and <em>NoSQL</em> databases? When would you pick each?`, "databases", "backend", "junior"},
	{"be-j-4", `Walk me through what happens when you call <em>git commit</em>. What does Git actually store?`, "tooling", "backend", "junior"},
	{"be-j-5", `What does <em>CI/CD</em> mean and why does it matter for backend services?`, "devops", "backend", "junior"},
	// backend – mid
	{"be-m-1", `Describe how you would design a <em>rate limiter</em> for a public API. What algorithm and where does state live?`, "system design", "backend", "mid"},
	{"be-m-2", `Explain <em>eventual consistency</em> and when you would accept it over strong consistency.`, "distributed systems", "backend", "mid"},
	{"be-m-3", `Walk me through how you would <em>debug a memory leak</em> in a production Go service.`, "backend", "backend", "mid"},
	{"be-m-4", `Trade-offs between <em>microservices</em> and a <em>monolith</em>: when is each the right call?`, "architecture", "backend", "mid"},
	{"be-m-5", `How does <em>connection pooling</em> work and what happens when the pool is exhausted?`, "databases", "backend", "mid"},
	// backend – senior
	{"be-s-1", `Design a <em>distributed caching layer</em> for 100k req/s. What are the failure modes (hot keys, stampede)?`, "system design", "backend", "senior"},
	{"be-s-2", `Explain the <em>CAP theorem</em> and how it influences your DB choice in a multi-region setup.`, "distributed systems", "backend", "senior"},
	{"be-s-3", `How do you guarantee <em>exactly-once processing</em> in an event-driven pipeline?`, "messaging", "backend", "senior"},
	{"be-s-4", `Describe a <em>significant architectural decision</em> you owned. What trade-offs did you evaluate?`, "leadership", "backend", "senior"},
	{"be-s-5", `How would you migrate a hot table from <em>Postgres</em> to a sharded store with zero downtime?`, "databases", "backend", "senior"},

	// frontend – junior
	{"fe-j-1", `What is the <em>virtual DOM</em> and how does it differ from the real DOM?`, "react", "frontend", "junior"},
	{"fe-j-2", `Explain <em>CSS specificity</em>. How do you resolve a conflict between two selectors?`, "css", "frontend", "junior"},
	{"fe-j-3", `What is the JavaScript <em>event loop</em>? Why is it called single-threaded?`, "javascript", "frontend", "junior"},
	{"fe-j-4", `When should you use <em>semantic HTML</em> vs generic divs? Give a concrete example.`, "html/a11y", "frontend", "junior"},
	{"fe-j-5", `Explain the difference between <em>async/await</em> and <em>promises</em>. What problems does each solve?`, "javascript", "frontend", "junior"},
	// frontend – mid
	{"fe-m-1", `Compare <em>client-side rendering</em>, <em>SSR</em>, and <em>SSG</em>. Pick a scenario for each.`, "rendering", "frontend", "mid"},
	{"fe-m-2", `How do you measure and improve <em>frontend performance</em>? Walk me through real metrics (LCP, INP, CLS).`, "performance", "frontend", "mid"},
	{"fe-m-3", `What problems does a state library like <em>Redux/Zustand</em> solve that React's <em>useState</em> doesn't?`, "state mgmt", "frontend", "mid"},
	{"fe-m-4", `How would you implement <em>accessibility</em> for a custom dropdown component?`, "a11y", "frontend", "mid"},
	{"fe-m-5", `Explain <em>code splitting</em> and when it actually helps (and when it hurts) load time.`, "performance", "frontend", "mid"},
	// frontend – senior
	{"fe-s-1", `Design a <em>component library</em> shared across multiple products. How do you version and document it?`, "design system", "frontend", "senior"},
	{"fe-s-2", `Compare a <em>micro-frontends</em> architecture with a monolithic SPA. What are the real costs?`, "architecture", "frontend", "senior"},
	{"fe-s-3", `How would you debug an <em>intermittent rendering bug</em> that only happens in production?`, "debugging", "frontend", "senior"},
	{"fe-s-4", `Build-time vs runtime performance: where should the bundle optimizer earn its keep?`, "performance", "frontend", "senior"},
	{"fe-s-5", `How do you design <em>frontend observability</em>? What signals does the team actually need?`, "observability", "frontend", "senior"},

	// fullstack – junior
	{"fs-j-1", `Walk me through what happens from typing <em>devspeak.ai</em> in the browser to seeing the page.`, "fundamentals", "fullstack", "junior"},
	{"fs-j-2", `What is <em>CORS</em> and why does the browser enforce it?`, "security", "fullstack", "junior"},
	{"fs-j-3", `What is the difference between <em>cookies</em>, <em>localStorage</em> and <em>sessionStorage</em>?`, "fundamentals", "fullstack", "junior"},
	// fullstack – mid
	{"fs-m-1", `Design the API contract for a <em>"my account" page</em>. What endpoints, what payloads, what status codes?`, "api design", "fullstack", "mid"},
	{"fs-m-2", `Compare <em>REST</em> and <em>GraphQL</em>. When does GraphQL pay off and when does it become a liability?`, "api design", "fullstack", "mid"},
	{"fs-m-3", `Explain the <em>N+1 query problem</em> from both DB and API design perspectives.`, "performance", "fullstack", "mid"},
	// fullstack – senior
	{"fs-s-1", `Design a <em>multi-tenant SaaS</em>. How do you isolate data, customize features, and stay sane?`, "architecture", "fullstack", "senior"},
	{"fs-s-2", `Where should <em>caching</em> live — CDN, gateway, app, DB? Walk through a real decision.`, "performance", "fullstack", "senior"},
	{"fs-s-3", `How do you roll out a <em>breaking API change</em> with zero downtime for web + mobile clients?`, "release", "fullstack", "senior"},

	// devops – junior
	{"do-j-1", `What is the difference between a <em>container</em> and a <em>virtual machine</em>?`, "containers", "devops", "junior"},
	{"do-j-2", `What is <em>infrastructure as code</em>? Why not just click in the AWS console?`, "iac", "devops", "junior"},
	{"do-j-3", `Explain the role of a <em>reverse proxy</em> like Nginx. Why put one in front of your app?`, "networking", "devops", "junior"},
	{"do-j-4", `What is a <em>Dockerfile</em>? What is the value of <em>multi-stage builds</em>?`, "containers", "devops", "junior"},
	// devops – mid
	{"do-m-1", `Compare a <em>Deployment</em>, <em>StatefulSet</em>, and <em>DaemonSet</em> in Kubernetes.`, "kubernetes", "devops", "mid"},
	{"do-m-2", `How does <em>Terraform state</em> work? What goes wrong when two engineers run apply at the same time?`, "iac", "devops", "mid"},
	{"do-m-3", `Walk me through a <em>blue/green deploy</em> on EC2 behind an ALB. What can break?`, "release", "devops", "mid"},
	{"do-m-4", `How do you wire <em>secrets</em> into a deployment without putting them in the repo?`, "security", "devops", "mid"},
	// devops – senior
	{"do-s-1", `Strategies for <em>zero-downtime deploys</em> on Kubernetes when running long-lived connections.`, "kubernetes", "devops", "senior"},
	{"do-s-2", `Compare <em>EKS</em>, <em>GKE</em>, and self-managed k8s. What are the real cost/complexity trade-offs?`, "cloud", "devops", "senior"},
	{"do-s-3", `Design an <em>observability stack</em> for a microservice fleet. Metrics, logs, traces — who owns what?`, "observability", "devops", "senior"},
	{"do-s-4", `How would you handle a <em>region-wide outage</em>? What does the runbook look like the next morning?`, "reliability", "devops", "senior"},

	// data – junior
	{"da-j-1", `Explain a <em>SQL JOIN</em> with a real example. Inner vs left vs full.`, "sql", "data", "junior"},
	{"da-j-2", `What is <em>data normalization</em>? Give a case where you would denormalize on purpose.`, "modeling", "data", "junior"},
	{"da-j-3", `What is the difference between <em>OLTP</em> and <em>OLAP</em>? Give examples.`, "fundamentals", "data", "junior"},
	// data – mid
	{"da-m-1", `Design a <em>daily ETL pipeline</em> that ingests 200 GB of events. How do you handle failures and reruns?`, "pipelines", "data", "mid"},
	{"da-m-2", `Explain <em>idempotency</em> in a streaming context. How do you avoid double-counting?`, "streaming", "data", "mid"},
	{"da-m-3", `What is a <em>columnar store</em>? Why is it fast for analytics?`, "storage", "data", "mid"},
	// data – senior
	{"da-s-1", `Design a <em>feature store</em> serving both training and online inference. What are the consistency rules?`, "ml infra", "data", "senior"},
	{"da-s-2", `Trade-offs between <em>batch</em> and <em>real-time</em>. Pick a use case where the wrong choice is a disaster.`, "pipelines", "data", "senior"},
	{"da-s-3", `How do you do <em>data lineage</em> at scale so that a broken dashboard maps back to the upstream pipeline?`, "governance", "data", "senior"},

	// mobile – junior
	{"mo-j-1", `Walk me through the <em>activity / view controller lifecycle</em> on your platform.`, "lifecycle", "mobile", "junior"},
	{"mo-j-2", `How do you handle <em>offline state</em> in a mobile app?`, "offline", "mobile", "junior"},
	{"mo-j-3", `What is the difference between <em>native</em>, <em>cross-platform</em>, and <em>hybrid</em> mobile?`, "fundamentals", "mobile", "junior"},
	// mobile – mid
	{"mo-m-1", `How do you manage <em>state across screens</em> in a non-trivial app?`, "state mgmt", "mobile", "mid"},
	{"mo-m-2", `Explain how <em>push notifications</em> actually work — from the server all the way to the locked phone.`, "platform", "mobile", "mid"},
	{"mo-m-3", `How do you debug a <em>memory leak</em> that only appears on certain devices?`, "debugging", "mobile", "mid"},
	// mobile – senior
	{"mo-s-1", `Design a <em>CI/CD pipeline</em> for a mobile app: signing, store review, staged rollout, rollback.`, "release", "mobile", "senior"},
	{"mo-s-2", `Cross-platform performance: where does <em>Flutter</em>/<em>React Native</em> actually struggle?`, "performance", "mobile", "senior"},
	{"mo-s-3", `How do you ship a <em>major refactor</em> behind a feature flag in a mobile codebase?`, "release", "mobile", "senior"},
}

// ──────────────────────────────────────────────────────────────────────────────
// Feedback canned (por nível) — placeholder até a integração com OpenAI
// ──────────────────────────────────────────────────────────────────────────────

var feedbackByLevel = map[string]evaluateResponse{
	"junior": {
		Score:      "7.5/10",
		ScoreClass: "score-ok",
		Technical:  "Good start — you covered the core idea. To push higher, be explicit about WHY each concept matters in production (not just what it is), and anchor your answer with one concrete real-world example you have shipped or used.",
		English: []string{
			`Consider "I would say that..." → more natural: "In simple terms,..."`,
			`"...makes it faster" → more precise: "...increases throughput"`,
			`Avoid starting sentences with "So" in a formal interview context.`,
		},
		Vocab: []string{
			`"handle multiple tasks" → "manage concurrent workloads"`,
			`"at the same time" → "simultaneously" or "in parallel"`,
			`"works well" → "performs efficiently" or "scales horizontally"`,
		},
	},
	"mid": {
		Score:      "8.2/10",
		ScoreClass: "score-good",
		Technical:  "Strong answer. You covered the core concepts and named a real trade-off. To push to a 9+, quantify (latency budgets, throughput targets) and discuss what you would measure in production to know your decision was correct.",
		English: []string{
			`Phrase "like, basically" → remove filler words for a more confident delivery`,
			`"we need to store" → "we maintain" or "we persist" (more formal)`,
			`Good use of "trade-off" — keep doing that, it signals seniority.`,
		},
		Vocab: []string{
			`"save counts" → "persist request counters"`,
			`"block the request" → "reject with HTTP 429 Too Many Requests"`,
			`"fast storage" → "low-latency data store (e.g., Redis)"`,
		},
	},
	"senior": {
		Score:      "9.0/10",
		ScoreClass: "score-good",
		Technical:  "Excellent. You covered the architecture, named the failure modes and tied the decision back to business constraints. To round it off, address rollback strategy and what observability signals would tell you the change is regressing.",
		English: []string{
			`Very fluent overall. Minor: "in terms of" appears repeatedly — vary with "regarding", "as for", "when it comes to"`,
			`"we talked about" → "as I mentioned" (more professional)`,
		},
		Vocab: []string{
			`"lots of requests" → "high-throughput workloads"`,
			`"cache misses piling up" → "cache stampede / thundering herd problem"`,
			`Good use of "hot key" and "eviction policy" — these are exactly the terms interviewers look for.`,
		},
	},
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func normalizeStack(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	allowed := map[string]bool{
		"backend": true, "frontend": true, "fullstack": true,
		"devops": true, "data": true, "mobile": true,
	}
	if allowed[s] {
		return s
	}
	return "backend"
}

func normalizeLevel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "junior" || s == "mid" || s == "senior" {
		return s
	}
	return "junior"
}

// scoreQuestion bias: keyword overlap entre topic+text e o targetRole do perfil
func scoreQuestion(q Question, keywords []string) int {
	if len(keywords) == 0 {
		return 0
	}
	hay := strings.ToLower(q.Text + " " + q.Topic)
	score := 0
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(hay, kw) {
			score++
		}
	}
	return score
}

func extractKeywords(targetRole string) []string {
	if targetRole == "" {
		return nil
	}
	raw := strings.FieldsFunc(strings.ToLower(targetRole), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "you": true, "your": true,
		"are": true, "have": true, "this": true, "that": true, "will": true, "from": true,
		"role": true, "job": true, "team": true, "work": true, "must": true, "should": true,
		"experience": true, "knowledge": true, "looking": true, "candidate": true,
	}
	out := make([]string, 0, len(raw))
	for _, w := range raw {
		if len(w) < 3 || stop[w] {
			continue
		}
		out = append(out, w)
	}
	return out
}

// pickQuestions filtra o banco pelo perfil e devolve até `limit` perguntas,
// priorizando as que casam com palavras-chave do targetRole.
func pickQuestions(p Profile, limit int) []Question {
	stack := normalizeStack(p.Stack)
	level := normalizeLevel(p.Level)
	keywords := extractKeywords(p.TargetRole)

	type scored struct {
		q     Question
		score int
		order int
	}
	pool := make([]scored, 0, 16)
	for i, q := range questionBank {
		if q.Stack != stack || q.Level != level {
			continue
		}
		pool = append(pool, scored{q: q, score: scoreQuestion(q, keywords), order: i})
	}
	// fallback: se a combinação (stack, level) não existe no banco, relaxa pra stack only
	if len(pool) == 0 {
		for i, q := range questionBank {
			if q.Stack != stack {
				continue
			}
			pool = append(pool, scored{q: q, score: scoreQuestion(q, keywords), order: i})
		}
	}
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].score != pool[j].score {
			return pool[i].score > pool[j].score
		}
		return pool[i].order < pool[j].order
	})
	if limit <= 0 || limit > len(pool) {
		limit = len(pool)
	}
	out := make([]Question, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, pool[i].q)
	}
	return out
}

// ──────────────────────────────────────────────────────────────────────────────
// HTTP handlers
// ──────────────────────────────────────────────────────────────────────────────

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func healthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("DevSpeak AI Speech Service Running"))
}

func evaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req evaluateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<17)).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	answer := strings.TrimSpace(req.Answer)
	if len(answer) < 50 {
		http.Error(w, "answer must be at least 50 characters", http.StatusBadRequest)
		return
	}

	var p Profile
	if req.Profile != nil {
		p = *req.Profile
	}
	if p.Level == "" {
		p.Level = req.Level
	}
	p.Stack = normalizeStack(p.Stack)
	p.Level = normalizeLevel(p.Level)

	var fb *evaluateResponse
	if ai != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if resp, err := ai.evaluate(ctx, p, req.Question, answer); err == nil {
			fb = resp
		} else {
			log.Printf("ai.evaluate failed, using canned fallback: %v", err)
		}
	}
	if fb == nil {
		c := feedbackByLevel[p.Level]
		fb = &c
	}

	// Persiste em background se o usuário está autenticado
	if uid, ok := userFromRequest(r); ok {
		go persistEvaluation(uid, p, req.Question, answer, fb)
	}

	writeJSON(w, http.StatusOK, fb)
}

func questions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req questionsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<17)).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Limit == 0 {
		req.Limit = 5
	}
	normalized := Profile{
		Name:            strings.TrimSpace(req.Profile.Name),
		Stack:           normalizeStack(req.Profile.Stack),
		Level:           normalizeLevel(req.Profile.Level),
		YearsExperience: req.Profile.YearsExperience,
		PrimaryLanguage: strings.TrimSpace(req.Profile.PrimaryLanguage),
		TargetRole:      strings.TrimSpace(req.Profile.TargetRole),
	}

	var qs []Question
	if ai != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		if generated, err := ai.questions(ctx, normalized, req.Limit); err == nil {
			qs = generated
		} else {
			log.Printf("ai.questions failed, falling back to bank: %v", err)
		}
	}
	if len(qs) == 0 {
		qs = pickQuestions(normalized, req.Limit)
	}
	writeJSON(w, http.StatusOK, questionsResponse{Questions: qs, Profile: normalized})
}

func staticDir() string {
	if d := os.Getenv("STATIC_DIR"); d != "" {
		return d
	}
	return "./public"
}

func main() {
	ai = newAIClient()
	initJWTSecret()
	if err := initDB(); err != nil {
		log.Printf("warn: %v — auth/history endpoints will be unavailable", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthCheck)
	mux.HandleFunc("/api/evaluate", optionalAuth(evaluate))
	mux.HandleFunc("/api/questions", optionalAuth(questions))
	mux.HandleFunc("/api/auth/signup", signup)
	mux.HandleFunc("/api/auth/login", login)
	mux.HandleFunc("/api/me", requireAuth(me))
	mux.HandleFunc("/api/me/update", requireAuth(updateMe))
	mux.HandleFunc("/api/history", requireAuth(history))
	mux.Handle("/metrics", promhttp.Handler())

	dir := staticDir()
	if _, err := os.Stat(dir); err == nil {
		mux.Handle("/", http.FileServer(http.Dir(dir)))
		log.Printf("serving static files from %s", dir)
	} else {
		log.Printf("static dir %q not found, skipping static serving", dir)
	}

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Println("Server running on port 8080")
	log.Fatal(srv.ListenAndServe())
}
