package repository

import (
	"context"
	"taskservice/internal/domain"
)

func (s *Store) CreateUser(ctx context.Context, u *domain.User) error {
	r, err := s.q.ExecContext(ctx, "INSERT INTO users (email,password_hash,name,created_at) VALUES (?,?,?,?)", u.Email, u.PasswordHash, u.Name, u.CreatedAt)
	if err != nil {
		return dbError(err)
	}
	u.ID, err = r.LastInsertId()
	return err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := s.q.QueryRowContext(ctx, "SELECT id,email,password_hash,name,created_at FROM users WHERE email = ?", email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt)
	return u, dbError(err)
}

func (s *Store) UserExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := s.q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", id).Scan(&exists)
	return exists, err
}

func (s *Store) CreateTeam(ctx context.Context, t *domain.Team) error {
	r, err := s.q.ExecContext(ctx, "INSERT INTO teams (name,created_by,created_at) VALUES (?,?,?)", t.Name, t.CreatedBy, t.CreatedAt)
	if err != nil {
		return dbError(err)
	}
	t.ID, err = r.LastInsertId()
	return err
}

func (s *Store) AddMember(ctx context.Context, m domain.Member) error {
	_, err := s.q.ExecContext(ctx, "INSERT INTO team_members (team_id,user_id,role) VALUES (?,?,?)", m.TeamID, m.UserID, m.Role)
	return dbError(err)
}

func (s *Store) TeamAccess(ctx context.Context, teamID, userID int64) (role string, version int64, err error) {
	err = s.q.QueryRowContext(ctx, `SELECT m.role,t.cache_version FROM team_members m JOIN teams t ON t.id=m.team_id WHERE m.team_id=? AND m.user_id=?`, teamID, userID).Scan(&role, &version)
	return role, version, dbError(err)
}

func (s *Store) LockTeam(ctx context.Context, id int64) error {
	var found int64
	return dbError(s.q.QueryRowContext(ctx, "SELECT id FROM teams WHERE id=? FOR UPDATE", id).Scan(&found))
}

func (s *Store) ChangeRole(ctx context.Context, m domain.Member) error {
	_, err := s.q.ExecContext(ctx, "UPDATE team_members SET role=? WHERE team_id=? AND user_id=?", m.Role, m.TeamID, m.UserID)
	return dbError(err)
}

func (s *Store) ListTeams(ctx context.Context, userID int64) ([]domain.Team, error) {
	rows, err := s.q.QueryContext(ctx, `SELECT t.id,t.name,t.created_by,t.created_at,m.role FROM teams t JOIN team_members m ON m.team_id=t.id WHERE m.user_id=? ORDER BY t.created_at DESC,t.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Team{}
	for rows.Next() {
		var t domain.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy, &t.CreatedAt, &t.Role); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// The cache namespace changes atomically with task writes, including when Redis
// is unavailable. Old list keys expire without a scan or a distributed commit.
func (s *Store) InvalidateTasks(ctx context.Context, teamID int64) error {
	_, err := s.q.ExecContext(ctx, "UPDATE teams SET cache_version=cache_version+1 WHERE id=?", teamID)
	return err
}
