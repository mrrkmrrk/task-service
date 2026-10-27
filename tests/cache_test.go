package tests

import (
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"taskservice/internal/cache"
	"taskservice/internal/domain"
	"taskservice/internal/service"
)

func TestCacheOutageAndLateWriter(t *testing.T) {
	f := setup(t)
	if f.redis == nil {
		t.Skip("TEST_REDIS_ADDR required")
	}
	owner := f.user(t, "cache-outage")
	team := f.team(t, owner)
	token := f.token(t, owner)
	task := call[domain.Task](t, f, "POST", "/api/v1/tasks", token, map[string]any{"team_id": team.ID, "title": "Cached title"}, 201)
	filter := domain.TaskFilter{TeamID: team.ID, Limit: 20}
	oldList, err := f.svc.ListTasks(f.ctx, owner.ID, filter)
	if err != nil {
		t.Fatal(err)
	}
	_, oldNamespace, err := f.repo.TeamAccess(f.ctx, team.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	// This service instance cannot reach Redis, while old keys remain in Redis.
	broken := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 10 * time.Millisecond, ReadTimeout: 10 * time.Millisecond, WriteTimeout: 10 * time.Millisecond, ContextTimeoutEnabled: true})
	t.Cleanup(func() { broken.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.New(f.repo, f.tokens, cache.New(broken, log))
	title := "Changed during Redis outage"
	updated, err := svc.UpdateTask(f.ctx, owner.ID, task.ID, service.UpdateTaskInput{Version: 1, Title: service.Optional[string]{Set: true, Value: &title}})
	if err != nil {
		t.Fatal(err)
	}
	fallback, err := svc.ListTasks(f.ctx, owner.ID, filter)
	if err != nil || len(fallback) != 1 || fallback[0].Title != title {
		t.Fatalf("database fallback: %+v %v", fallback, err)
	}
	// A delayed request can still write its old result, but only into the old namespace.
	cache.New(f.redis, log).Put(f.ctx, filter, oldNamespace, oldList)
	items := call[[]domain.Task](t, f, "GET", fmt.Sprintf("/api/v1/tasks?team_id=%d", team.ID), token, nil, 200)
	if len(items) != 1 || items[0].Version != updated.Version || items[0].Title != title {
		t.Fatalf("stale cache after recovery: %+v", items)
	}
}
