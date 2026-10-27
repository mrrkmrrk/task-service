package service

import (
	"encoding/json"
	"taskservice/internal/domain"
	"testing"
)

func TestTaskEditPermissions(t *testing.T) {
	assignee := int64(2)
	task := domain.Task{CreatedBy: 1, AssigneeID: &assignee}
	title := Optional[string]{Set: true}
	status := Optional[string]{Set: true}
	for _, tt := range []struct {
		name, role string
		user       int64
		input      UpdateTaskInput
		allowed    bool
	}{
		{"owner edits any field", "owner", 9, UpdateTaskInput{Title: title}, true},
		{"admin edits any field", "admin", 9, UpdateTaskInput{Title: title}, true},
		{"creator edits title", "member", 1, UpdateTaskInput{Title: title}, true},
		{"creator reassigns", "member", 1, UpdateTaskInput{AssigneeID: Optional[int64]{Set: true}}, true},
		{"assignee changes status", "member", 2, UpdateTaskInput{Status: status}, true},
		{"assignee cannot edit title", "member", 2, UpdateTaskInput{Title: title}, false},
		{"assignee cannot edit description", "member", 2, UpdateTaskInput{Description: Optional[string]{Set: true}}, false},
		{"assignee cannot reassign", "member", 2, UpdateTaskInput{AssigneeID: Optional[int64]{Set: true}}, false},
		{"unrelated member", "member", 3, UpdateTaskInput{Status: status}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := canEdit(tt.role, tt.user, task, tt.input); got != tt.allowed {
				t.Fatalf("got %v, want %v", got, tt.allowed)
			}
		})
	}
}

func TestOptionalAssignee(t *testing.T) {
	for _, tt := range []struct {
		input string
		set   bool
		id    *int64
	}{
		{`{"version":1}`, false, nil},
		{`{"version":1,"assignee_id":null}`, true, nil},
		{`{"version":1,"assignee_id":7}`, true, ptr(int64(7))},
	} {
		var in UpdateTaskInput
		if err := json.Unmarshal([]byte(tt.input), &in); err != nil {
			t.Fatal(err)
		}
		if in.AssigneeID.Set != tt.set || !equalID(in.AssigneeID.Value, tt.id) {
			t.Fatalf("wrong decoding: %+v", in.AssigneeID)
		}
	}
}
func ptr[T any](v T) *T { return &v }
