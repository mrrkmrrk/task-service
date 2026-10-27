package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"taskservice/docs"
	"taskservice/internal/auth"
	"taskservice/internal/domain"
	"taskservice/internal/service"
)

type API struct {
	service *service.Service
	tokens  *auth.Tokens
	log     *slog.Logger
	ready   func(context.Context) error
}
type handler func(http.ResponseWriter, *http.Request) error
type userKey struct{}

func New(s *service.Service, t *auth.Tokens, log *slog.Logger, ready func(context.Context) error) http.Handler {
	a := &API{s, t, log, ready}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/register", a.wrap(a.register))
	mux.HandleFunc("POST /api/v1/login", a.wrap(a.login))
	routes := map[string]handler{
		"POST /api/v1/teams": a.createTeam, "GET /api/v1/teams": a.listTeams,
		"POST /api/v1/teams/{id}/invite":            a.invite,
		"PATCH /api/v1/teams/{id}/members/{userID}": a.changeRole,
		"POST /api/v1/tasks":                        a.createTask, "GET /api/v1/tasks": a.listTasks,
		"PUT /api/v1/tasks/{id}":           a.updateTask,
		"GET /api/v1/tasks/{id}/history":   a.history,
		"POST /api/v1/tasks/{id}/comments": a.addComment,
		"GET /api/v1/tasks/{id}/comments":  a.comments,
		"GET /api/v1/teams/{id}/stats":     a.stats,
	}
	for route, h := range routes {
		mux.HandleFunc(route, a.wrap(a.protect(h)))
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := a.ready(ctx); err != nil {
			a.log.Error("readiness failed", "error", err)
			writeError(w, 503, "unavailable", "service is not ready")
			return
		}
		write(w, 200, map[string]string{"status": "ready"})
	})
	mux.Handle("GET /swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(docs.Files))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { writeError(w, 404, "not_found", "endpoint not found") })
	return a.observe(mux)
}

func (a *API) wrap(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			a.failure(w, err)
		}
	}
}

func (a *API) protect(next handler) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return domain.ErrUnauthorized
		}
		id, err := a.tokens.Verify(parts[1])
		if err != nil {
			return err
		}
		return next(w, r.WithContext(context.WithValue(r.Context(), userKey{}, id)))
	}
}
func userID(r *http.Request) int64 { return r.Context().Value(userKey{}).(int64) }

func body[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var input T
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return input, domain.Invalid("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, domain.Invalid("invalid JSON body or unknown field")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return input, domain.Invalid("body must contain one JSON object")
	}
	return input, nil
}
func pathID(r *http.Request, name string) (int64, error) {
	n, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || n <= 0 {
		return 0, domain.Invalid(name + " must be a positive integer")
	}
	return n, nil
}
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func (a *API) failure(w http.ResponseWriter, err error) {
	var validation *domain.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, 400, "bad_request", validation.Message)
	case errors.Is(err, domain.ErrUnauthorized):
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, 401, "unauthorized", domain.ErrUnauthorized.Error())
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, 403, "forbidden", domain.ErrForbidden.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, 404, "not_found", domain.ErrNotFound.Error())
	case errors.Is(err, domain.ErrVersion):
		writeError(w, 409, "version_conflict", domain.ErrVersion.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, 409, "conflict", domain.ErrConflict.Error())
	default:
		a.log.Error("request failed", "error", err)
		writeError(w, 500, "internal_error", "internal server error")
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (a *API) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w}
		rw.Header().Set("X-Content-Type-Options", "nosniff")
		rw.Header().Set("Cache-Control", "no-store")
		defer func() {
			if recovered := recover(); recovered != nil {
				a.log.Error("request panic")
				if rw.status == 0 {
					writeError(rw, 500, "internal_error", "internal server error")
				}
			}
			a.log.Info("http request", "method", r.Method, "path", r.URL.Path, "status", rw.status, "duration_ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(rw, r)
	})
}
