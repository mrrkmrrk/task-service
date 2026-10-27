package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"taskservice/internal/auth"
	"taskservice/internal/cache"
	"taskservice/internal/domain"
	"taskservice/internal/repository"
	"taskservice/internal/service"
	httpapi "taskservice/internal/transport/http"
	"taskservice/migrations"
)

type fixture struct {
	db     *sql.DB
	repo   *repository.Store
	svc    *service.Service
	tokens *auth.Tokens
	redis  *redis.Client
	server *httptest.Server
	ctx    context.Context
}

func setup(t *testing.T) *fixture {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN or run docker compose -f docker-compose.test.yml")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	f := &fixture{db: db, repo: repository.New(db), tokens: auth.New("integration-secret-at-least-thirty-two-bytes", time.Hour), ctx: ctx}
	var tasks *cache.Tasks
	if addr := os.Getenv("TEST_REDIS_ADDR"); addr != "" {
		f.redis = redis.NewClient(&redis.Options{Addr: addr, MaxRetries: -1})
		t.Cleanup(func() { f.redis.Close() })
		if err := f.redis.Ping(ctx).Err(); err != nil {
			t.Fatal(err)
		}
		tasks = cache.New(f.redis, log)
	}
	f.svc = service.New(f.repo, f.tokens, tasks)
	f.server = httptest.NewServer(httpapi.New(f.svc, f.tokens, log, db.PingContext))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fixture) user(t *testing.T, name string) domain.User {
	t.Helper()
	u, err := f.svc.Register(f.ctx, service.RegisterInput{Email: fmt.Sprintf("%s-%d@example.test", name, time.Now().UnixNano()), Password: "test-password", Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func (f *fixture) team(t *testing.T, owner domain.User) domain.Team {
	t.Helper()
	team, err := f.svc.CreateTeam(f.ctx, owner.ID, service.TeamInput{Name: "Integration team"})
	if err != nil {
		t.Fatal(err)
	}
	return team
}

func (f *fixture) invite(t *testing.T, owner domain.User, team domain.Team, user domain.User, role string) {
	t.Helper()
	if _, err := f.svc.Invite(f.ctx, owner.ID, team.ID, service.InviteInput{UserID: user.ID, Role: role}); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) token(t *testing.T, u domain.User) string {
	t.Helper()
	token, err := f.tokens.Issue(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func request(f *fixture, method, path, token string, input any) (int, []byte, error) {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(f.ctx, method, f.server.URL+path, body)
	if err != nil {
		return 0, nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp.StatusCode, data, err
}

func call[T any](t *testing.T, f *fixture, method, path, token string, input any, want int) T {
	t.Helper()
	status, data, err := request(f, method, path, token, input)
	if err != nil {
		t.Fatal(err)
	}
	if status != want {
		t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, status, want, data)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid response: %s: %v", data, err)
	}
	return result
}
