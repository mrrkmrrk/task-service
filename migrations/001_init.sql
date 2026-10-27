CREATE TABLE IF NOT EXISTS users (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(254) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE KEY users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS teams (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    created_by BIGINT NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    cache_version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT teams_creator FOREIGN KEY (created_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS team_members (
    team_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    role ENUM('owner','admin','member') NOT NULL,
    PRIMARY KEY (team_id, user_id),
    KEY members_user_team (user_id, team_id),
    CONSTRAINT members_team FOREIGN KEY (team_id) REFERENCES teams(id),
    CONSTRAINT members_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS tasks (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    team_id BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    status ENUM('todo','in_progress','done') NOT NULL,
    created_by BIGINT NOT NULL,
    assignee_id BIGINT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    closed_at DATETIME(6) NULL,
    version BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT tasks_team FOREIGN KEY (team_id) REFERENCES teams(id),
    CONSTRAINT tasks_creator FOREIGN KEY (created_by) REFERENCES users(id),
    CONSTRAINT tasks_assignee_member FOREIGN KEY (team_id, assignee_id) REFERENCES team_members(team_id, user_id),
    CONSTRAINT tasks_version_positive CHECK (version > 0),
    CONSTRAINT tasks_closed_status CHECK ((status = 'done' AND closed_at IS NOT NULL) OR (status <> 'done' AND closed_at IS NULL)),
    KEY tasks_team_status (team_id, status),
    KEY tasks_team_assignee (team_id, assignee_id),
    KEY tasks_team_created (team_id, created_at, id),
    KEY tasks_assignee_closed (assignee_id, closed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS task_history (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    changed_by BIGINT NOT NULL,
    changes JSON NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT history_task FOREIGN KEY (task_id) REFERENCES tasks(id),
    CONSTRAINT history_user FOREIGN KEY (changed_by) REFERENCES users(id),
    KEY history_task_created (task_id, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS task_comments (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT comments_task FOREIGN KEY (task_id) REFERENCES tasks(id),
    CONSTRAINT comments_user FOREIGN KEY (user_id) REFERENCES users(id),
    KEY comments_task_created (task_id, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
