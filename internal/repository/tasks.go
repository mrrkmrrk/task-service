package repository

import (
	"context"
	"encoding/json"
	"taskservice/internal/domain"
	"time"
)

const taskColumns = "id,team_id,title,description,status,created_by,assignee_id,created_at,updated_at,closed_at,version"

type scanner interface{ Scan(...any) error }

func scanTask(row scanner) (domain.Task, error) {
	var t domain.Task
	err := row.Scan(&t.ID, &t.TeamID, &t.Title, &t.Description, &t.Status, &t.CreatedBy, &t.AssigneeID, &t.CreatedAt, &t.UpdatedAt, &t.ClosedAt, &t.Version)
	return t, dbError(err)
}

func (s *Store) Task(ctx context.Context, id int64) (domain.Task, error) {
	return scanTask(s.q.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id=?", id))
}

func (s *Store) CreateTask(ctx context.Context, t *domain.Task) error {
	r, err := s.q.ExecContext(ctx, `INSERT INTO tasks (team_id,title,description,status,created_by,assignee_id,created_at,updated_at,closed_at,version) VALUES (?,?,?,?,?,?,?,?,?,?)`, t.TeamID, t.Title, t.Description, t.Status, t.CreatedBy, t.AssigneeID, t.CreatedAt, t.UpdatedAt, t.ClosedAt, t.Version)
	if err != nil {
		return dbError(err)
	}
	t.ID, err = r.LastInsertId()
	return err
}

func (s *Store) UpdateTask(ctx context.Context, t domain.Task, expected int64) error {
	r, err := s.q.ExecContext(ctx, `UPDATE tasks SET title=?,description=?,status=?,assignee_id=?,updated_at=?,closed_at=?,version=version+1 WHERE id=? AND version=?`, t.Title, t.Description, t.Status, t.AssigneeID, t.UpdatedAt, t.ClosedAt, t.ID, expected)
	if err != nil {
		return dbError(err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		if _, err := s.Task(ctx, t.ID); err != nil {
			return err
		}
		return domain.ErrVersion
	}
	return nil
}

func (s *Store) ListTasks(ctx context.Context, f domain.TaskFilter) ([]domain.Task, error) {
	query := "SELECT " + taskColumns + " FROM tasks WHERE team_id=?"
	args := []any{f.TeamID}
	if f.Status != "" {
		query += " AND status=?"
		args = append(args, f.Status)
	}
	if f.AssigneeID != nil {
		query += " AND assignee_id=?"
		args = append(args, *f.AssigneeID)
	}
	query += " ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?"
	args = append(args, f.Limit, f.Offset)
	rows, err := s.q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (s *Store) AddHistory(ctx context.Context, taskID, userID int64, changes map[string]domain.Change, at time.Time) error {
	data, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	_, err = s.q.ExecContext(ctx, "INSERT INTO task_history (task_id,changed_by,changes,created_at) VALUES (?,?,?,?)", taskID, userID, data, at)
	return dbError(err)
}

func (s *Store) History(ctx context.Context, taskID int64) ([]domain.History, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT id,task_id,changed_by,changes,created_at FROM task_history WHERE task_id=? ORDER BY created_at ASC,id ASC", taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.History{}
	for rows.Next() {
		var h domain.History
		if err := rows.Scan(&h.ID, &h.TaskID, &h.ChangedBy, &h.Changes, &h.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, h)
	}
	return items, rows.Err()
}

func (s *Store) AddComment(ctx context.Context, c *domain.Comment) error {
	r, err := s.q.ExecContext(ctx, "INSERT INTO task_comments (task_id,user_id,content,created_at) VALUES (?,?,?,?)", c.TaskID, c.UserID, c.Content, c.CreatedAt)
	if err != nil {
		return dbError(err)
	}
	c.ID, err = r.LastInsertId()
	return err
}

func (s *Store) Comments(ctx context.Context, taskID int64) ([]domain.Comment, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT id,task_id,user_id,content,created_at FROM task_comments WHERE task_id=? ORDER BY created_at ASC,id ASC", taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Comment{}
	for rows.Next() {
		var c domain.Comment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.UserID, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
