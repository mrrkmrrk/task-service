WITH team_tasks AS (
    SELECT id, status, assignee_id, created_at, closed_at
    FROM tasks WHERE team_id = ?
), totals AS (
    SELECT COALESCE(SUM(status = 'todo'), 0) AS todo,
           COALESCE(SUM(status = 'in_progress'), 0) AS in_progress,
           COALESCE(SUM(status = 'done'), 0) AS done,
           COALESCE(AVG(CASE WHEN closed_at IS NOT NULL
               THEN TIMESTAMPDIFF(MICROSECOND, created_at, closed_at) / 1000000.0 END), 0) AS avg_close_seconds
    FROM team_tasks
), top_assignees AS (
    SELECT u.id AS user_id, u.name, COUNT(*) AS closed_tasks
    FROM team_tasks t JOIN users u ON u.id = t.assignee_id
    WHERE t.closed_at >= UTC_TIMESTAMP(6) - INTERVAL 30 DAY
      AND t.closed_at <= UTC_TIMESTAMP(6)
    GROUP BY u.id, u.name
    ORDER BY closed_tasks DESC, u.id ASC
    LIMIT 3
), comments AS (
    SELECT COUNT(*) AS comments_count
    FROM task_comments c JOIN team_tasks t ON t.id = c.task_id
)
SELECT totals.todo, totals.in_progress, totals.done, totals.avg_close_seconds,
       comments.comments_count, a.user_id, a.name, a.closed_tasks
FROM totals CROSS JOIN comments LEFT JOIN top_assignees a ON TRUE
ORDER BY a.closed_tasks DESC, a.user_id ASC
