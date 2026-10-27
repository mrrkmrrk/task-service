package tests

import (
	"fmt"
	"taskservice/internal/domain"
	"testing"
	"time"
)

func TestTaskHistoryAtomicity(t *testing.T) {
	f := setup(t)
	owner := f.user(t, "atomic")
	team := f.team(t, owner)
	token := f.token(t, owner)
	task := call[domain.Task](t, f, "POST", "/api/v1/tasks", token, map[string]any{"team_id": team.ID, "title": "Before"}, 201)
	_, namespace, err := f.repo.TeamAccess(f.ctx, team.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	trigger := fmt.Sprintf("reject_history_%d", time.Now().UnixNano())
	_, err = f.db.ExecContext(f.ctx, "CREATE TRIGGER "+trigger+" BEFORE INSERT ON task_history FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'intentional history test failure'")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := f.db.ExecContext(f.ctx, "DROP TRIGGER IF EXISTS "+trigger); err != nil {
			t.Error(err)
		}
	})
	call[any](t, f, "PUT", fmt.Sprintf("/api/v1/tasks/%d", task.ID), token, map[string]any{"version": 1, "title": "Must roll back"}, 500)
	call[any](t, f, "POST", "/api/v1/tasks", token, map[string]any{"team_id": team.ID, "title": "Must not exist"}, 500)
	persisted, err := f.repo.Task(f.ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Title != "Before" || persisted.Version != 1 {
		t.Fatalf("task was not rolled back: %+v", persisted)
	}
	history, err := f.repo.History(f.ctx, task.ID)
	if err != nil || len(history) != 1 {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	items, err := f.repo.ListTasks(f.ctx, domain.TaskFilter{TeamID: team.ID, Limit: 20})
	if err != nil || len(items) != 1 {
		t.Fatalf("create rollback: %+v %v", items, err)
	}
	_, after, err := f.repo.TeamAccess(f.ctx, team.ID, owner.ID)
	if err != nil || after != namespace {
		t.Fatalf("cache namespace not rolled back: %d -> %d, %v", namespace, after, err)
	}
}
