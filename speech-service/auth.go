package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type ctxKey string

const userIDKey ctxKey = "userID"

var jwtSigningKey []byte

func initJWTSecret() {
	s := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if s == "" {
		// fallback dev: chave aleatória por processo. Tokens emitidos invalidam
		// no próximo restart — bom pra dev, ruim pra prod (precisa setar JWT_SECRET).
		buf := make([]byte, 32)
		_, _ = rand.Read(buf)
		s = hex.EncodeToString(buf)
		log.Println("auth: JWT_SECRET not set — generated ephemeral key (tokens invalidate on restart). Set JWT_SECRET in .env for production.")
	}
	jwtSigningKey = []byte(s)
}

func issueToken(userID int64) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(jwtSigningKey)
}

func parseToken(tok string) (int64, error) {
	parsed, err := jwt.Parse(tok, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSigningKey, nil
	})
	if err != nil {
		return 0, err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return 0, errors.New("invalid token")
	}
	sub, ok := claims["sub"].(float64)
	if !ok {
		return 0, errors.New("invalid sub claim")
	}
	return int64(sub), nil
}

// ──────────────────────────────────────────────────────────────────────────────
// DTOs
// ──────────────────────────────────────────────────────────────────────────────

type signupRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	Name            string `json:"name"`
	Stack           string `json:"stack"`
	Level           string `json:"level"`
	YearsExperience int    `json:"yearsExperience"`
	PrimaryLanguage string `json:"primaryLanguage"`
	TargetRole      string `json:"targetRole"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userPublic struct {
	ID              int64  `json:"id"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	Stack           string `json:"stack"`
	Level           string `json:"level"`
	YearsExperience int    `json:"yearsExperience"`
	PrimaryLanguage string `json:"primaryLanguage"`
	TargetRole      string `json:"targetRole"`
}

type authResponse struct {
	Token string     `json:"token"`
	User  userPublic `json:"user"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Handlers
// ──────────────────────────────────────────────────────────────────────────────

func signup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if db == nil {
		http.Error(w, "auth unavailable: db not initialized", http.StatusServiceUnavailable)
		return
	}
	var req signupRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !strings.Contains(email, "@") {
		http.Error(w, "invalid email", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 6 {
		http.Error(w, "password must be at least 6 characters", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	stack := normalizeStack(req.Stack)
	level := normalizeLevel(req.Level)
	name := strings.TrimSpace(req.Name)
	lang := strings.TrimSpace(req.PrimaryLanguage)
	target := strings.TrimSpace(req.TargetRole)

	var id int64
	err = db.QueryRowContext(r.Context(), `
		INSERT INTO users(email, password_hash, name, stack, level, primary_language, target_role, years_experience)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id
	`, email, string(hash), name, stack, level, lang, target, req.YearsExperience).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			http.Error(w, "email already registered", http.StatusConflict)
			return
		}
		log.Printf("signup db: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	tok, err := issueToken(id)
	if err != nil {
		log.Printf("signup token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{
		Token: tok,
		User: userPublic{
			ID: id, Email: email, Name: name, Stack: stack, Level: level,
			YearsExperience: req.YearsExperience, PrimaryLanguage: lang, TargetRole: target,
		},
	})
}

func login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if db == nil {
		http.Error(w, "auth unavailable: db not initialized", http.StatusServiceUnavailable)
		return
	}
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	var u userPublic
	var hash string
	err := db.QueryRowContext(r.Context(), `
		SELECT id, email, password_hash, name, stack, level, years_experience, primary_language, target_role
		FROM users WHERE email=$1
	`, email).Scan(&u.ID, &u.Email, &hash, &u.Name, &u.Stack, &u.Level,
		&u.YearsExperience, &u.PrimaryLanguage, &u.TargetRole)
	if err != nil {
		// custo similar a uma comparação bcrypt válida pra reduzir oracle de timing
		_, _ = bcrypt.GenerateFromPassword([]byte("dummy"), bcrypt.MinCost)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	tok, err := issueToken(u.ID)
	if err != nil {
		log.Printf("login token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: tok, User: u})
}

func me(w http.ResponseWriter, r *http.Request) {
	id, ok := userFromRequest(r)
	if !ok || db == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var u userPublic
	err := db.QueryRowContext(r.Context(), `
		SELECT id, email, name, stack, level, years_experience, primary_language, target_role
		FROM users WHERE id=$1
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.Stack, &u.Level,
		&u.YearsExperience, &u.PrimaryLanguage, &u.TargetRole)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func updateMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := userFromRequest(r)
	if !ok || db == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req signupRequest // mesma shape, sem password
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	stack := normalizeStack(req.Stack)
	level := normalizeLevel(req.Level)
	_, err := db.ExecContext(r.Context(), `
		UPDATE users SET name=$1, stack=$2, level=$3, primary_language=$4, target_role=$5, years_experience=$6
		WHERE id=$7
	`, strings.TrimSpace(req.Name), stack, level,
		strings.TrimSpace(req.PrimaryLanguage), strings.TrimSpace(req.TargetRole),
		req.YearsExperience, id)
	if err != nil {
		log.Printf("updateMe: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	me(w, r)
}

// ──────────────────────────────────────────────────────────────────────────────
// Middleware
// ──────────────────────────────────────────────────────────────────────────────

func userFromRequest(r *http.Request) (int64, bool) {
	v := r.Context().Value(userIDKey)
	id, ok := v.(int64)
	return id, ok
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func optionalAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if tok := extractToken(r); tok != "" {
			if id, err := parseToken(tok); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), userIDKey, id))
			}
		}
		h(w, r)
	}
}

func requireAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := extractToken(r)
		if tok == "" {
			http.Error(w, "auth required", http.StatusUnauthorized)
			return
		}
		id, err := parseToken(tok)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), userIDKey, id))
		h(w, r)
	}
}
