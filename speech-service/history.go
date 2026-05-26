package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

type historyItem struct {
	ID         int64    `json:"id"`
	Question   string   `json:"question"`
	Answer     string   `json:"answer"`
	Score      string   `json:"score"`
	ScoreClass string   `json:"scoreClass"`
	Technical  string   `json:"technical"`
	English    []string `json:"english"`
	Vocab      []string `json:"vocab"`
	Level      string   `json:"level"`
	Stack      string   `json:"stack"`
	CreatedAt  string   `json:"createdAt"`
}

func persistEvaluation(userID int64, profile Profile, question, answer string, fb *evaluateResponse) {
	if db == nil {
		return
	}
	engJSON, _ := json.Marshal(fb.English)
	vocJSON, _ := json.Marshal(fb.Vocab)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `
		INSERT INTO evaluations(user_id, question, answer, score, score_class, technical, english, vocab, level, stack)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, userID, question, answer, fb.Score, fb.ScoreClass, fb.Technical, engJSON, vocJSON, profile.Level, profile.Stack)
	if err != nil {
		log.Printf("persist evaluation: %v", err)
	}
}

func history(w http.ResponseWriter, r *http.Request) {
	if db == nil {
		http.Error(w, "history unavailable", http.StatusServiceUnavailable)
		return
	}
	uid, ok := userFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	rows, err := db.QueryContext(r.Context(), `
		SELECT id, question, answer, score, score_class, technical, english, vocab, level, stack, created_at
		FROM evaluations WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2
	`, uid, limit)
	if err != nil {
		log.Printf("history query: %v", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := []historyItem{}
	for rows.Next() {
		var it historyItem
		var engJSON, vocJSON []byte
		var createdAt time.Time
		if err := rows.Scan(&it.ID, &it.Question, &it.Answer, &it.Score, &it.ScoreClass,
			&it.Technical, &engJSON, &vocJSON, &it.Level, &it.Stack, &createdAt); err != nil {
			log.Printf("history scan: %v", err)
			continue
		}
		_ = json.Unmarshal(engJSON, &it.English)
		_ = json.Unmarshal(vocJSON, &it.Vocab)
		it.CreatedAt = createdAt.Format(time.RFC3339)
		out = append(out, it)
	}
	writeJSON(w, http.StatusOK, out)
}
