package tests

import (
	"fmt"
	"math"
	"taskservice/internal/domain"
	"testing"
	"time"
)

func TestSQLReport(t *testing.T) {
	f := setup(t)
	users := []domain.User{f.user(t, "report-a"), f.user(t, "report-b"), f.user(t, "report-c"), f.user(t, "report-d")}
	team := f.team(t, users[0])
	for _, u := range users[1:] {
		f.invite(t, users[0], team, u, "member")
	}
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	seed := func(teamID int64, status string, assignee *int64, age int, duration time.Duration) int64 {
		t.Helper()
		closed := base.AddDate(0, 0, -age)
		created := closed.Add(-duration)
		var closedAt *time.Time
		if status == "done" {
			closedAt = &closed
		}
		task := domain.Task{TeamID: teamID, Title: "Report fixture", Status: status, CreatedBy: users[0].ID, AssigneeID: assignee, CreatedAt: created, UpdatedAt: closed, ClosedAt: closedAt, Version: 1}
		if err := f.repo.CreateTask(f.ctx, &task); err != nil {
			t.Fatal(err)
		}
		return task.ID
	}
	var taskIDs []int64
	for i, count := range []int{4, 3, 2, 1} {
		for n := 0; n < count; n++ {
			taskIDs = append(taskIDs, seed(team.ID, "done", &users[i].ID, 1, time.Duration(i+1)*time.Hour))
		}
	}
	seed(team.ID, "done", &users[3].ID, 40, 5*time.Hour)
	seed(team.ID, "done", nil, 2, 6*time.Hour)
	seed(team.ID, "todo", nil, 0, 0)
	seed(team.ID, "todo", nil, 0, 0)
	seed(team.ID, "in_progress", &users[1].ID, 0, 0)
	for i := 0; i < 5; i++ {
		comment := domain.Comment{TaskID: taskIDs[i%2], UserID: users[0].ID, Content: "comment", CreatedAt: base}
		if err := f.repo.AddComment(f.ctx, &comment); err != nil {
			t.Fatal(err)
		}
	}
	other := f.team(t, users[0])
	f.invite(t, users[0], other, users[3], "member")
	for i := 0; i < 8; i++ {
		id := seed(other.ID, "done", &users[3].ID, 1, 100*time.Hour)
		for j := 0; j < 7; j++ {
			c := domain.Comment{TaskID: id, UserID: users[0].ID, Content: "foreign comment", CreatedAt: base}
			if err := f.repo.AddComment(f.ctx, &c); err != nil {
				t.Fatal(err)
			}
		}
	}
	stats, err := f.repo.Stats(f.ctx, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TasksByStatus["todo"] != 2 || stats.TasksByStatus["in_progress"] != 1 || stats.TasksByStatus["done"] != 12 {
		t.Fatalf("wrong status totals: %+v", stats)
	}
	if len(stats.TopAssignees) != 3 {
		t.Fatalf("expected top 3: %+v", stats.TopAssignees)
	}
	for i, top := range stats.TopAssignees {
		if top.UserID != users[i].ID || top.Name != users[i].Name || top.ClosedTasks != int64(4-i) {
			t.Fatalf("wrong ranking: %+v", stats.TopAssignees)
		}
	}
	if math.Abs(stats.AvgCloseSeconds-9300) > 0.001 {
		t.Fatalf("avg=%f want=9300", stats.AvgCloseSeconds)
	}
	if stats.CommentsCount != 5 {
		t.Fatalf("comments=%d want=5", stats.CommentsCount)
	}
	api := call[domain.Stats](t, f, "GET", fmt.Sprintf("/api/v1/teams/%d/stats", team.ID), f.token(t, users[0]), nil, 200)
	if api.CommentsCount != 5 || api.TasksByStatus["done"] != 12 {
		t.Fatalf("wrong HTTP stats: %+v", api)
	}
	call[any](t, f, "GET", fmt.Sprintf("/api/v1/teams/%d/stats", team.ID), f.token(t, users[1]), nil, 403)
	outsider := f.user(t, "report-outsider")
	call[any](t, f, "GET", fmt.Sprintf("/api/v1/teams/%d/stats", team.ID), f.token(t, outsider), nil, 403)
	empty := f.team(t, users[0])
	zero, err := f.repo.Stats(f.ctx, empty.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(zero.TopAssignees) != 0 || zero.TopAssignees == nil || zero.AvgCloseSeconds != 0 || zero.CommentsCount != 0 || zero.TasksByStatus["done"] != 0 || zero.TasksByStatus["todo"] != 0 || zero.TasksByStatus["in_progress"] != 0 {
		t.Fatalf("empty team: %+v", zero)
	}
	// Equal counts have a deterministic user_id tie-breaker, including at rank 3.
	tied := f.team(t, users[0])
	for _, u := range users[1:] {
		f.invite(t, users[0], tied, u, "member")
	}
	for _, u := range users {
		seed(tied.ID, "done", &u.ID, 1, time.Hour)
	}
	ties, err := f.repo.Stats(f.ctx, tied.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i, top := range ties.TopAssignees {
		if top.UserID != users[i].ID {
			t.Fatalf("unstable tie order: %+v", ties.TopAssignees)
		}
	}
}
