package httpapi

import (
	"net/http"
	"net/url"
	"strconv"
	"taskservice/internal/domain"
	"taskservice/internal/service"
)

func (a *API) register(w http.ResponseWriter, r *http.Request) error {
	in, err := body[service.RegisterInput](w, r)
	if err != nil {
		return err
	}
	u, err := a.service.Register(r.Context(), in)
	if err != nil {
		return err
	}
	write(w, 201, u)
	return nil
}
func (a *API) login(w http.ResponseWriter, r *http.Request) error {
	in, err := body[service.LoginInput](w, r)
	if err != nil {
		return err
	}
	token, err := a.service.Login(r.Context(), in)
	if err != nil {
		return err
	}
	write(w, 200, token)
	return nil
}
func (a *API) createTeam(w http.ResponseWriter, r *http.Request) error {
	in, err := body[service.TeamInput](w, r)
	if err != nil {
		return err
	}
	t, err := a.service.CreateTeam(r.Context(), userID(r), in)
	if err != nil {
		return err
	}
	write(w, 201, t)
	return nil
}
func (a *API) listTeams(w http.ResponseWriter, r *http.Request) error {
	t, err := a.service.ListTeams(r.Context(), userID(r))
	if err != nil {
		return err
	}
	write(w, 200, t)
	return nil
}
func (a *API) invite(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	in, err := body[service.InviteInput](w, r)
	if err != nil {
		return err
	}
	m, err := a.service.Invite(r.Context(), userID(r), id, in)
	if err != nil {
		return err
	}
	write(w, 201, m)
	return nil
}
func (a *API) changeRole(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	target, err := pathID(r, "userID")
	if err != nil {
		return err
	}
	in, err := body[service.RoleInput](w, r)
	if err != nil {
		return err
	}
	m, err := a.service.ChangeRole(r.Context(), userID(r), id, target, in)
	if err != nil {
		return err
	}
	write(w, 200, m)
	return nil
}
func (a *API) createTask(w http.ResponseWriter, r *http.Request) error {
	in, err := body[service.CreateTaskInput](w, r)
	if err != nil {
		return err
	}
	t, err := a.service.CreateTask(r.Context(), userID(r), in)
	if err != nil {
		return err
	}
	write(w, 201, t)
	return nil
}
func (a *API) updateTask(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	in, err := body[service.UpdateTaskInput](w, r)
	if err != nil {
		return err
	}
	t, err := a.service.UpdateTask(r.Context(), userID(r), id, in)
	if err != nil {
		return err
	}
	write(w, 200, t)
	return nil
}
func (a *API) listTasks(w http.ResponseWriter, r *http.Request) error {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return domain.Invalid("invalid query string")
	}
	for key, values := range query {
		if len(values) != 1 {
			return domain.Invalid("query parameters must not repeat")
		}
		switch key {
		case "team_id", "status", "assignee_id", "limit", "offset":
		default:
			return domain.Invalid("unknown query parameter")
		}
	}
	f := domain.TaskFilter{Status: query.Get("status"), Limit: 20}
	f.TeamID, err = strconv.ParseInt(query.Get("team_id"), 10, 64)
	if err != nil || f.TeamID <= 0 {
		return domain.Invalid("team_id is required and must be positive")
	}
	if query.Has("assignee_id") {
		id, e := strconv.ParseInt(query.Get("assignee_id"), 10, 64)
		if e != nil || id <= 0 {
			return domain.Invalid("assignee_id must be positive")
		}
		f.AssigneeID = &id
	}
	if query.Has("limit") {
		f.Limit, err = strconv.Atoi(query.Get("limit"))
		if err != nil {
			return domain.Invalid("invalid limit")
		}
	}
	if query.Has("offset") {
		f.Offset, err = strconv.Atoi(query.Get("offset"))
		if err != nil {
			return domain.Invalid("invalid offset")
		}
	}
	tasks, err := a.service.ListTasks(r.Context(), userID(r), f)
	if err != nil {
		return err
	}
	write(w, 200, tasks)
	return nil
}
func (a *API) history(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	h, err := a.service.History(r.Context(), userID(r), id)
	if err != nil {
		return err
	}
	write(w, 200, h)
	return nil
}
func (a *API) addComment(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	in, err := body[service.CommentInput](w, r)
	if err != nil {
		return err
	}
	c, err := a.service.AddComment(r.Context(), userID(r), id, in)
	if err != nil {
		return err
	}
	write(w, 201, c)
	return nil
}
func (a *API) comments(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	c, err := a.service.Comments(r.Context(), userID(r), id)
	if err != nil {
		return err
	}
	write(w, 200, c)
	return nil
}
func (a *API) stats(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	s, err := a.service.Stats(r.Context(), userID(r), id)
	if err != nil {
		return err
	}
	write(w, 200, s)
	return nil
}
