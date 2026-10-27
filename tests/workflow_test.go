package tests

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"taskservice/internal/cache"
	"taskservice/internal/domain"
)

func TestAPIWorkflow(t *testing.T) {
	f := setup(t)
	if f.redis == nil {
		t.Skip("TEST_REDIS_ADDR required for cache integration")
	}
	email := fmt.Sprintf("http-owner-%d@example.test", time.Now().UnixNano())
	registration := map[string]any{"email": email, "password": "password123", "name": "Owner"}
	owner := call[domain.User](t, f, "POST", "/api/v1/register", "", registration, 201)
	call[any](t, f, "POST", "/api/v1/register", "", registration, 409)
	call[any](t, f, "POST", "/api/v1/register", "", map[string]any{"email": "bad", "password": "short", "name": ""}, 400)
	login := call[map[string]string](t, f, "POST", "/api/v1/login", "", map[string]any{"email": email, "password": "password123"}, 200)
	ownerToken := login["access_token"]
	if ownerToken == "" {
		t.Fatal("missing token")
	}
	call[any](t, f, "POST", "/api/v1/login", "", map[string]any{"email": email, "password": "incorrect"}, 401)
	member, assignee, admin, outsider := f.user(t, "creator"), f.user(t, "assignee"), f.user(t, "admin"), f.user(t, "outsider")
	memberToken, assigneeToken, adminToken, outsiderToken := f.token(t, member), f.token(t, assignee), f.token(t, admin), f.token(t, outsider)
	call[any](t, f, "GET", "/api/v1/teams", "", nil, 401)
	call[any](t, f, "GET", "/api/v1/teams", "bad-token", nil, 401)
	team := call[domain.Team](t, f, "POST", "/api/v1/teams", ownerToken, map[string]any{"name": "HTTP team"}, 201)
	invitePath := fmt.Sprintf("/api/v1/teams/%d/invite", team.ID)
	for _, u := range []domain.User{member, assignee} {
		call[domain.Member](t, f, "POST", invitePath, ownerToken, map[string]any{"user_id": u.ID, "role": "member"}, 201)
	}
	call[domain.Member](t, f, "POST", invitePath, ownerToken, map[string]any{"user_id": admin.ID, "role": "admin"}, 201)
	call[any](t, f, "POST", invitePath, ownerToken, map[string]any{"user_id": member.ID, "role": "admin"}, 409)
	call[any](t, f, "POST", invitePath, ownerToken, map[string]any{"user_id": outsider.ID, "role": "owner"}, 400)
	call[any](t, f, "POST", invitePath, memberToken, map[string]any{"user_id": outsider.ID, "role": "member"}, 403)
	call[any](t, f, "POST", invitePath, ownerToken, map[string]any{"user_id": 9223372036854775807, "role": "member"}, 404)
	call[any](t, f, "POST", invitePath, adminToken, map[string]any{"user_id": owner.ID, "role": "member"}, 409)
	ownerRolePath := fmt.Sprintf("/api/v1/teams/%d/members/%d", team.ID, owner.ID)
	call[any](t, f, "PATCH", ownerRolePath, adminToken, map[string]any{"role": "member"}, 403)
	call[any](t, f, "PATCH", ownerRolePath, ownerToken, map[string]any{"role": "admin"}, 403)
	memberRolePath := fmt.Sprintf("/api/v1/teams/%d/members/%d", team.ID, member.ID)
	call[any](t, f, "PATCH", memberRolePath, ownerToken, map[string]any{"role": "admin"}, 200)
	call[any](t, f, "PATCH", memberRolePath, ownerToken, map[string]any{"role": "member"}, 200)
	if teams := call[[]domain.Team](t, f, "GET", "/api/v1/teams", outsiderToken, nil, 200); len(teams) != 0 {
		t.Fatal("foreign teams leaked")
	}
	if teams := call[[]domain.Team](t, f, "GET", "/api/v1/teams", memberToken, nil, 200); len(teams) != 1 || teams[0].ID != team.ID {
		t.Fatalf("missing own team: %+v", teams)
	}
	input := map[string]any{"team_id": team.ID, "title": "Original", "description": "Details", "assignee_id": assignee.ID}
	task := call[domain.Task](t, f, "POST", "/api/v1/tasks", memberToken, input, 201)
	if task.CreatedBy != member.ID || task.Version != 1 || task.Status != "todo" || task.ClosedAt != nil {
		t.Fatalf("bad created task: %+v", task)
	}
	input["created_by"] = owner.ID
	call[any](t, f, "POST", "/api/v1/tasks", memberToken, input, 400)
	delete(input, "created_by")
	input["assignee_id"] = outsider.ID
	call[any](t, f, "POST", "/api/v1/tasks", memberToken, input, 400)
	input["assignee_id"] = assignee.ID
	call[any](t, f, "POST", "/api/v1/tasks", outsiderToken, input, 403)
	listPath := fmt.Sprintf("/api/v1/tasks?team_id=%d", team.ID)
	items := call[[]domain.Task](t, f, "GET", listPath, memberToken, nil, 200)
	if len(items) != 1 || items[0].ID != task.ID {
		t.Fatalf("wrong list: %+v", items)
	}
	_, namespace, err := f.repo.TeamAccess(f.ctx, team.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	filter := domain.TaskFilter{TeamID: team.ID, Limit: 20}
	key := cache.Key(filter, namespace)
	ttl, err := f.redis.TTL(f.ctx, key).Result()
	if err != nil || ttl < 290*time.Second || ttl > 300*time.Second {
		t.Fatalf("cache TTL=%s err=%v", ttl, err)
	}
	before, err := f.redis.Info(f.ctx, "stats").Result()
	if err != nil {
		t.Fatal(err)
	}
	call[[]domain.Task](t, f, "GET", listPath, memberToken, nil, 200)
	after, err := f.redis.Info(f.ctx, "stats").Result()
	if err != nil {
		t.Fatal(err)
	}
	if statCounter(after, "keyspace_hits") <= statCounter(before, "keyspace_hits") {
		t.Fatal("second list did not hit Redis")
	}
	call[any](t, f, "GET", listPath, outsiderToken, nil, 403)
	for _, path := range []string{"/api/v1/tasks", listPath + "&limit=101", listPath + "&offset=-1", listPath + "&status=bad", listPath + "&limit=0", listPath + "&assignee_id=0", listPath + "&limit=2&limit=3"} {
		call[any](t, f, "GET", path, memberToken, nil, 400)
	}
	path := fmt.Sprintf("/api/v1/tasks/%d", task.ID)
	for _, suffix := range []string{"/history", "/comments"} {
		call[any](t, f, "GET", path+suffix, outsiderToken, nil, 404)
	}
	call[any](t, f, "PUT", path, outsiderToken, map[string]any{"version": 1, "status": "done"}, 404)
	call[any](t, f, "POST", path+"/comments", outsiderToken, map[string]any{"content": "hidden"}, 404)
	call[any](t, f, "PUT", path, assigneeToken, map[string]any{"version": 1, "title": "forbidden"}, 403)
	call[any](t, f, "PUT", path, assigneeToken, map[string]any{"version": 1, "description": "forbidden"}, 403)
	call[any](t, f, "PUT", path, assigneeToken, map[string]any{"version": 1, "assignee_id": member.ID}, 403)
	call[any](t, f, "PUT", path, memberToken, map[string]any{"version": 1, "assignee_id": outsider.ID}, 400)
	call[any](t, f, "PUT", path, memberToken, map[string]any{"version": 1, "title": nil}, 400)
	task = call[domain.Task](t, f, "PUT", path, assigneeToken, map[string]any{"version": 1, "status": "done"}, 200)
	if task.Version != 2 || task.ClosedAt == nil {
		t.Fatalf("bad close: %+v", task)
	}
	call[any](t, f, "PUT", path, memberToken, map[string]any{"version": 1, "title": "stale"}, 409)
	items = call[[]domain.Task](t, f, "GET", listPath, memberToken, nil, 200)
	if items[0].Version != 2 || items[0].Status != "done" {
		t.Fatal("cache was not invalidated")
	}
	if items := call[[]domain.Task](t, f, "GET", listPath+"&status=todo", memberToken, nil, 200); len(items) != 0 {
		t.Fatal("status filter ignored")
	}
	if items := call[[]domain.Task](t, f, "GET", listPath+fmt.Sprintf("&status=done&assignee_id=%d&limit=1&offset=0", assignee.ID), memberToken, nil, 200); len(items) != 1 {
		t.Fatal("assignee/status filter")
	}
	if items := call[[]domain.Task](t, f, "GET", listPath+"&limit=1&offset=1", memberToken, nil, 200); len(items) != 0 {
		t.Fatal("pagination ignored")
	}
	history := call[[]domain.History](t, f, "GET", path+"/history", memberToken, nil, 200)
	if len(history) != 2 || history[1].ChangedBy != assignee.ID {
		t.Fatalf("wrong history: %+v", history)
	}
	var changes map[string]domain.Change
	if err := json.Unmarshal(history[1].Changes, &changes); err != nil {
		t.Fatal(err)
	}
	if changes["status"].Old != "todo" || changes["status"].New != "done" {
		t.Fatalf("wrong old/new values: %s", history[1].Changes)
	}
	task = call[domain.Task](t, f, "PUT", path, memberToken, map[string]any{"version": 2, "status": "done"}, 200)
	if task.Version != 2 {
		t.Fatal("no-op incremented version")
	}
	if history := call[[]domain.History](t, f, "GET", path+"/history", memberToken, nil, 200); len(history) != 2 {
		t.Fatal("no-op history written")
	}
	task = call[domain.Task](t, f, "PUT", path, adminToken, map[string]any{"version": 2, "status": "in_progress", "assignee_id": nil, "title": "Admin edit"}, 200)
	if task.ClosedAt != nil || task.AssigneeID != nil || task.Version != 3 {
		t.Fatalf("reopen/unassign: %+v", task)
	}
	call[any](t, f, "POST", path+"/comments", memberToken, map[string]any{"content": "   "}, 400)
	c := call[domain.Comment](t, f, "POST", path+"/comments", assigneeToken, map[string]any{"content": "Looks good"}, 201)
	if c.UserID != assignee.ID {
		t.Fatal("wrong comment author")
	}
	if comments := call[[]domain.Comment](t, f, "GET", path+"/comments", memberToken, nil, 200); len(comments) != 1 || comments[0].ID != c.ID {
		t.Fatal("wrong comments")
	}
	call[any](t, f, "GET", fmt.Sprintf("/api/v1/teams/%d/stats", team.ID), memberToken, nil, 403)
	call[domain.Stats](t, f, "GET", fmt.Sprintf("/api/v1/teams/%d/stats", team.ID), adminToken, nil, 200)
	call[domain.Stats](t, f, "GET", fmt.Sprintf("/api/v1/teams/%d/stats", team.ID), ownerToken, nil, 200)
	// A member who is neither creator nor assignee cannot edit.
	call[any](t, f, "PUT", path, assigneeToken, map[string]any{"version": 3, "status": "done"}, 403)
	second := call[domain.Task](t, f, "POST", "/api/v1/tasks", ownerToken, map[string]any{"team_id": team.ID, "title": "Created done", "status": "done"}, 201)
	if second.ClosedAt == nil {
		t.Fatal("creation with done did not set closed_at")
	}
	items = call[[]domain.Task](t, f, "GET", listPath, memberToken, nil, 200)
	if len(items) != 2 || items[0].ID != second.ID {
		t.Fatal("create invalidation or stable order failed")
	}
	status, data, err := request(f, "GET", "/swagger/", "", nil)
	if err != nil || status != 200 || !strings.Contains(string(data), "SwaggerUIBundle") {
		t.Fatalf("swagger page: %d %v", status, err)
	}
	call[any](t, f, "GET", "/swagger/openapi.json", "", nil, 200)
}

func statCounter(info, key string) int64 {
	for _, line := range strings.Split(info, "\n") {
		if strings.HasPrefix(line, key+":") {
			var n int64
			fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, key+":")), "%d", &n)
			return n
		}
	}
	return -1
}

func TestConcurrentUpdates(t *testing.T) {
	f := setup(t)
	owner := f.user(t, "concurrent")
	team := f.team(t, owner)
	token := f.token(t, owner)
	task := call[domain.Task](t, f, "POST", "/api/v1/tasks", token, map[string]any{"team_id": team.ID, "title": "Initial"}, 201)
	path := fmt.Sprintf("/api/v1/tasks/%d", task.ID)
	type result struct {
		status int
		data   []byte
		err    error
	}
	results := make(chan result, 12)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			status, data, err := request(f, "PUT", path, token, map[string]any{"version": 1, "title": fmt.Sprintf("Winner %d", i)})
			results <- result{status, data, err}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	var winner domain.Task
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		switch result.status {
		case 200:
			success++
			if err := json.Unmarshal(result.data, &winner); err != nil {
				t.Fatal(err)
			}
		case 409:
			conflicts++
		default:
			t.Fatalf("unexpected concurrent response %d: %s", result.status, result.data)
		}
	}
	if success != 1 || conflicts != 11 {
		t.Fatalf("success=%d conflicts=%d", success, conflicts)
	}
	history := call[[]domain.History](t, f, "GET", path+"/history", token, nil, 200)
	if len(history) != 2 {
		t.Fatalf("history count=%d", len(history))
	}
	var changes map[string]domain.Change
	if err := json.Unmarshal(history[1].Changes, &changes); err != nil {
		t.Fatal(err)
	}
	if changes["title"].Old != "Initial" || changes["title"].New != winner.Title {
		t.Fatalf("history not from winning version: %s", history[1].Changes)
	}
	// Parallel creation must not deadlock when updating the team's namespace.
	created := make(chan result, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, data, err := request(f, "POST", "/api/v1/tasks", token, map[string]any{"team_id": team.ID, "title": "Parallel creation"})
			created <- result{status, data, err}
		}()
	}
	wg.Wait()
	close(created)
	for r := range created {
		if r.err != nil || r.status != 201 {
			t.Fatalf("parallel create: %d %s %v", r.status, r.data, r.err)
		}
	}
}
