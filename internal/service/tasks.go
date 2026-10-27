package service

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"taskservice/internal/domain"
	"taskservice/internal/repository"
)

type CreateTaskInput struct {
	TeamID      int64  `json:"team_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	AssigneeID  *int64 `json:"assignee_id"`
}

// Optional distinguishes an omitted field from an explicit null (unassign).
type Optional[T any] struct {
	Set   bool
	Value *T
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

type UpdateTaskInput struct {
	Version     int64            `json:"version"`
	Title       Optional[string] `json:"title"`
	Description Optional[string] `json:"description"`
	Status      Optional[string] `json:"status"`
	AssigneeID  Optional[int64]  `json:"assignee_id"`
}

func validateTask(t domain.Task) error {
	if !textValid(t.Title, 255) {
		return domain.Invalid("title must contain 1 to 255 characters")
	}
	if !utf8.ValidString(t.Description) || len(t.Description) > 16000 {
		return domain.Invalid("description must be valid text up to 16000 bytes")
	}
	if !domain.ValidStatus(t.Status) {
		return domain.Invalid("status must be todo, in_progress or done")
	}
	return nil
}

func (s *Service) CreateTask(ctx context.Context, userID int64, in CreateTaskInput) (domain.Task, error) {
	if in.Status == "" {
		in.Status = "todo"
	}
	at := now()
	t := domain.Task{TeamID: in.TeamID, Title: strings.TrimSpace(in.Title), Description: in.Description, Status: in.Status, AssigneeID: in.AssigneeID, CreatedBy: userID, CreatedAt: at, UpdatedAt: at, Version: 1}
	if t.Status == "done" {
		t.ClosedAt = &at
	}
	if err := validateTask(t); err != nil {
		return t, err
	}
	err := s.store.WithinTx(ctx, func(q *repository.Store) error {
		if _, _, err := access(ctx, q, t.TeamID, userID); err != nil {
			return err
		}
		if err := checkAssignee(ctx, q, t.TeamID, t.AssigneeID); err != nil {
			return err
		}
		if err := q.InvalidateTasks(ctx, t.TeamID); err != nil {
			return err
		}
		if err := q.CreateTask(ctx, &t); err != nil {
			return err
		}
		changes := map[string]domain.Change{
			"team_id": {Old: nil, New: t.TeamID}, "title": {Old: nil, New: t.Title},
			"description": {Old: nil, New: t.Description}, "status": {Old: nil, New: t.Status},
			"created_by": {Old: nil, New: userID}, "created_at": {Old: nil, New: at},
			"updated_at": {Old: nil, New: at}, "version": {Old: nil, New: t.Version},
		}
		if t.AssigneeID != nil {
			changes["assignee_id"] = domain.Change{Old: nil, New: t.AssigneeID}
		}
		if t.ClosedAt != nil {
			changes["closed_at"] = domain.Change{Old: nil, New: t.ClosedAt}
		}
		return q.AddHistory(ctx, t.ID, userID, changes, at)
	})
	return t, err
}

func canEdit(role string, userID int64, t domain.Task, in UpdateTaskInput) bool {
	if domain.Manager(role) || t.CreatedBy == userID {
		return true
	}
	return t.AssigneeID != nil && *t.AssigneeID == userID && !in.Title.Set && !in.Description.Set && !in.AssigneeID.Set
}

func equalID(a, b *int64) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

func (s *Service) UpdateTask(ctx context.Context, userID, taskID int64, in UpdateTaskInput) (domain.Task, error) {
	var result domain.Task
	if in.Version <= 0 {
		return result, domain.Invalid("positive version required")
	}
	if !in.Title.Set && !in.Description.Set && !in.Status.Set && !in.AssigneeID.Set {
		return result, domain.Invalid("at least one editable field required")
	}
	if (in.Title.Set && in.Title.Value == nil) || (in.Description.Set && in.Description.Value == nil) || (in.Status.Set && in.Status.Value == nil) {
		return result, domain.Invalid("only assignee_id can be null")
	}
	err := s.store.WithinTx(ctx, func(q *repository.Store) error {
		old, role, err := s.visibleTask(ctx, q, userID, taskID)
		if err != nil {
			return err
		}
		if !canEdit(role, userID, old, in) {
			return domain.ErrForbidden
		}
		if old.Version != in.Version {
			return domain.ErrVersion
		}
		updated := old
		if in.Title.Set {
			updated.Title = strings.TrimSpace(*in.Title.Value)
		}
		if in.Description.Set {
			updated.Description = *in.Description.Value
		}
		if in.Status.Set {
			updated.Status = *in.Status.Value
		}
		if in.AssigneeID.Set {
			updated.AssigneeID = in.AssigneeID.Value
		}
		if err := validateTask(updated); err != nil {
			return err
		}
		if err := checkAssignee(ctx, q, updated.TeamID, updated.AssigneeID); err != nil {
			return err
		}
		changes := map[string]domain.Change{}
		if old.Title != updated.Title {
			changes["title"] = domain.Change{Old: old.Title, New: updated.Title}
		}
		if old.Description != updated.Description {
			changes["description"] = domain.Change{Old: old.Description, New: updated.Description}
		}
		if old.Status != updated.Status {
			changes["status"] = domain.Change{Old: old.Status, New: updated.Status}
		}
		if !equalID(old.AssigneeID, updated.AssigneeID) {
			changes["assignee_id"] = domain.Change{Old: old.AssigneeID, New: updated.AssigneeID}
		}
		if len(changes) == 0 {
			result = old
			return nil
		}
		updated.UpdatedAt = now()
		updated.Version = old.Version + 1
		if old.Status != updated.Status {
			if updated.Status == "done" {
				at := updated.UpdatedAt
				updated.ClosedAt = &at
			} else {
				updated.ClosedAt = nil
			}
			if old.ClosedAt != nil || updated.ClosedAt != nil {
				changes["closed_at"] = domain.Change{Old: old.ClosedAt, New: updated.ClosedAt}
			}
		}
		changes["version"] = domain.Change{Old: old.Version, New: updated.Version}
		changes["updated_at"] = domain.Change{Old: old.UpdatedAt, New: updated.UpdatedAt}
		if err := q.InvalidateTasks(ctx, updated.TeamID); err != nil {
			return err
		}
		if err := q.UpdateTask(ctx, updated, in.Version); err != nil {
			return err
		}
		if err := q.AddHistory(ctx, taskID, userID, changes, updated.UpdatedAt); err != nil {
			return err
		}
		result = updated
		return nil
	})
	return result, err
}

func (s *Service) ListTasks(ctx context.Context, userID int64, f domain.TaskFilter) ([]domain.Task, error) {
	if f.Limit < 1 || f.Limit > 100 || f.Offset < 0 {
		return nil, domain.Invalid("limit must be 1..100 and offset must be nonnegative")
	}
	if f.Status != "" && !domain.ValidStatus(f.Status) {
		return nil, domain.Invalid("invalid status")
	}
	if f.AssigneeID != nil && *f.AssigneeID <= 0 {
		return nil, domain.Invalid("assignee_id must be positive")
	}
	_, version, err := access(ctx, s.store, f.TeamID, userID)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		if tasks, ok := s.cache.Get(ctx, f, version); ok {
			return tasks, nil
		}
	}
	tasks, err := s.store.ListTasks(ctx, f)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		s.cache.Put(ctx, f, version, tasks)
	}
	return tasks, nil
}

func (s *Service) History(ctx context.Context, userID, taskID int64) ([]domain.History, error) {
	if _, _, err := s.visibleTask(ctx, s.store, userID, taskID); err != nil {
		return nil, err
	}
	return s.store.History(ctx, taskID)
}

type CommentInput struct {
	Content string `json:"content"`
}

func (s *Service) AddComment(ctx context.Context, userID, taskID int64, in CommentInput) (domain.Comment, error) {
	c := domain.Comment{TaskID: taskID, UserID: userID, Content: strings.TrimSpace(in.Content), CreatedAt: now()}
	if !textValid(c.Content, 4000) {
		return c, domain.Invalid("comment must contain 1 to 4000 characters")
	}
	if _, _, err := s.visibleTask(ctx, s.store, userID, taskID); err != nil {
		return c, err
	}
	err := s.store.AddComment(ctx, &c)
	return c, err
}

func (s *Service) Comments(ctx context.Context, userID, taskID int64) ([]domain.Comment, error) {
	if _, _, err := s.visibleTask(ctx, s.store, userID, taskID); err != nil {
		return nil, err
	}
	return s.store.Comments(ctx, taskID)
}
