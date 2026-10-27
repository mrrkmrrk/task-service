package repository

import (
	"context"
	"database/sql"
	_ "embed"
	"taskservice/internal/domain"
)

//go:embed stats.sql
var statsQuery string

func (s *Store) Stats(ctx context.Context, teamID int64) (domain.Stats, error) {
	result := domain.Stats{TasksByStatus: map[string]int64{"todo": 0, "in_progress": 0, "done": 0}, TopAssignees: []domain.TopAssignee{}}
	rows, err := s.q.QueryContext(ctx, statsQuery, teamID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var todo, progress, done int64
		var id, count sql.NullInt64
		var name sql.NullString
		if err := rows.Scan(&todo, &progress, &done, &result.AvgCloseSeconds, &result.CommentsCount, &id, &name, &count); err != nil {
			return result, err
		}
		result.TasksByStatus["todo"] = todo
		result.TasksByStatus["in_progress"] = progress
		result.TasksByStatus["done"] = done
		if id.Valid {
			result.TopAssignees = append(result.TopAssignees, domain.TopAssignee{UserID: id.Int64, Name: name.String, ClosedTasks: count.Int64})
		}
	}
	return result, rows.Err()
}
