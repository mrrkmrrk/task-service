package service

import (
	"context"
	"strings"
	"taskservice/internal/domain"
	"taskservice/internal/repository"
)

type TeamInput struct {
	Name string `json:"name"`
}
type InviteInput struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
}
type RoleInput struct {
	Role string `json:"role"`
}

func (s *Service) CreateTeam(ctx context.Context, userID int64, in TeamInput) (domain.Team, error) {
	in.Name = strings.TrimSpace(in.Name)
	if !textValid(in.Name, 100) {
		return domain.Team{}, domain.Invalid("team name must contain 1 to 100 characters")
	}
	t := domain.Team{Name: in.Name, CreatedBy: userID, CreatedAt: now(), Role: "owner"}
	err := s.store.WithinTx(ctx, func(q *repository.Store) error {
		if err := q.CreateTeam(ctx, &t); err != nil {
			return err
		}
		return q.AddMember(ctx, domain.Member{TeamID: t.ID, UserID: userID, Role: "owner"})
	})
	return t, err
}

func (s *Service) ListTeams(ctx context.Context, userID int64) ([]domain.Team, error) {
	return s.store.ListTeams(ctx, userID)
}

func (s *Service) Invite(ctx context.Context, userID, teamID int64, in InviteInput) (domain.Member, error) {
	m := domain.Member{TeamID: teamID, UserID: in.UserID, Role: in.Role}
	if in.UserID <= 0 || (in.Role != "admin" && in.Role != "member") {
		return m, domain.Invalid("positive user_id and role admin/member required")
	}
	err := s.store.WithinTx(ctx, func(q *repository.Store) error {
		role, _, err := access(ctx, q, teamID, userID)
		if err != nil {
			return err
		}
		if !domain.Manager(role) {
			return domain.ErrForbidden
		}
		if err := q.LockTeam(ctx, teamID); err != nil {
			return err
		}
		role, _, err = access(ctx, q, teamID, userID)
		if err != nil {
			return err
		}
		if !domain.Manager(role) {
			return domain.ErrForbidden
		}
		exists, err := q.UserExists(ctx, in.UserID)
		if err != nil {
			return err
		}
		if !exists {
			return domain.ErrNotFound
		}
		return q.AddMember(ctx, m)
	})
	return m, err
}

func (s *Service) ChangeRole(ctx context.Context, userID, teamID, targetID int64, in RoleInput) (domain.Member, error) {
	m := domain.Member{TeamID: teamID, UserID: targetID, Role: in.Role}
	if targetID <= 0 || (in.Role != "admin" && in.Role != "member") {
		return m, domain.Invalid("positive user id and role admin/member required")
	}
	err := s.store.WithinTx(ctx, func(q *repository.Store) error {
		role, _, err := access(ctx, q, teamID, userID)
		if err != nil {
			return err
		}
		if role != "owner" {
			return domain.ErrForbidden
		}
		if err := q.LockTeam(ctx, teamID); err != nil {
			return err
		}
		targetRole, _, err := q.TeamAccess(ctx, teamID, targetID)
		if err != nil {
			return err
		}
		if targetRole == "owner" {
			return domain.ErrForbidden
		}
		return q.ChangeRole(ctx, m)
	})
	return m, err
}

func (s *Service) Stats(ctx context.Context, userID, teamID int64) (domain.Stats, error) {
	role, _, err := access(ctx, s.store, teamID, userID)
	if err != nil {
		return domain.Stats{}, err
	}
	if !domain.Manager(role) {
		return domain.Stats{}, domain.ErrForbidden
	}
	return s.store.Stats(ctx, teamID)
}
