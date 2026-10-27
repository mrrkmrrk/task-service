package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"taskservice/internal/auth"
	"taskservice/internal/cache"
	"taskservice/internal/domain"
	"taskservice/internal/repository"
)

type Service struct {
	store  *repository.Store
	tokens *auth.Tokens
	cache  *cache.Tasks
}

func New(store *repository.Store, tokens *auth.Tokens, cache *cache.Tasks) *Service {
	return &Service{store, tokens, cache}
}

func now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

func textValid(s string, max int) bool {
	return strings.TrimSpace(s) != "" && utf8.ValidString(s) && utf8.RuneCountInString(s) <= max
}

func access(ctx context.Context, q *repository.Store, teamID, userID int64) (string, int64, error) {
	if teamID <= 0 || userID <= 0 {
		return "", 0, domain.Invalid("ids must be positive")
	}
	role, version, err := q.TeamAccess(ctx, teamID, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return "", 0, domain.ErrForbidden
	}
	return role, version, err
}

func checkAssignee(ctx context.Context, q *repository.Store, teamID int64, id *int64) error {
	if id == nil {
		return nil
	}
	if *id <= 0 {
		return domain.Invalid("assignee_id must be positive or null")
	}
	_, _, err := q.TeamAccess(ctx, teamID, *id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Invalid("assignee must belong to the team")
	}
	return err
}

func (s *Service) visibleTask(ctx context.Context, q *repository.Store, userID, taskID int64) (domain.Task, string, error) {
	if taskID <= 0 {
		return domain.Task{}, "", domain.Invalid("task id must be positive")
	}
	t, err := q.Task(ctx, taskID)
	if err != nil {
		return t, "", err
	}
	role, _, err := access(ctx, q, t.TeamID, userID)
	// Hide whether a task exists outside the caller's teams.
	if errors.Is(err, domain.ErrForbidden) {
		return domain.Task{}, "", domain.ErrNotFound
	}
	return t, role, err
}
