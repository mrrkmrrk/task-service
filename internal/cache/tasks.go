package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"taskservice/internal/domain"
)

type Tasks struct {
	client *redis.Client
	log    *slog.Logger
}

func New(client *redis.Client, log *slog.Logger) *Tasks { return &Tasks{client, log} }

func Key(f domain.TaskFilter, version int64) string {
	assignee := "any"
	if f.AssigneeID != nil {
		assignee = strconv.FormatInt(*f.AssigneeID, 10)
	}
	return fmt.Sprintf("tasks:team:%d:v:%d:status:%s:assignee:%s:limit:%d:offset:%d", f.TeamID, version, f.Status, assignee, f.Limit, f.Offset)
}

func (c *Tasks) Get(ctx context.Context, f domain.TaskFilter, version int64) ([]domain.Task, bool) {
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	data, err := c.client.Get(ctx, Key(f, version)).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.log.Warn("task cache read failed", "error", err)
		}
		return nil, false
	}
	var tasks []domain.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		c.log.Warn("invalid task cache entry", "error", err)
		return nil, false
	}
	if tasks == nil {
		return nil, false
	}
	return tasks, true
}

func (c *Tasks) Put(ctx context.Context, f domain.TaskFilter, version int64, tasks []domain.Task) {
	data, err := json.Marshal(tasks)
	if err != nil {
		c.log.Error("encode task cache", "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	if err := c.client.Set(ctx, Key(f, version), data, 5*time.Minute).Err(); err != nil {
		c.log.Warn("task cache write failed", "error", err)
	}
}
