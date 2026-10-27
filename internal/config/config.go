package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

type Config struct {
	HTTPPort      string
	DSN           string
	RedisAddr     string
	RedisPassword string
	JWTSecret     string
	JWTTTL        time.Duration
}

func Load() (Config, error) {
	c := Config{HTTPPort: value("HTTP_PORT", "8080"), RedisAddr: value("REDIS_ADDR", "localhost:6379"), RedisPassword: os.Getenv("REDIS_PASSWORD"), JWTSecret: os.Getenv("JWT_SECRET")}
	port, err := strconv.Atoi(c.HTTPPort)
	if err != nil || port < 1 || port > 65535 {
		return c, fmt.Errorf("HTTP_PORT must be between 1 and 65535")
	}
	if len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("JWT_SECRET must contain at least 32 bytes")
	}
	c.JWTTTL, err = time.ParseDuration(value("JWT_TTL", "24h"))
	if err != nil || c.JWTTTL < time.Second {
		return c, fmt.Errorf("JWT_TTL must be a duration of at least 1s")
	}
	if os.Getenv("MYSQL_PASSWORD") == "" {
		return c, fmt.Errorf("MYSQL_PASSWORD is required")
	}
	mysqlPort := value("MYSQL_PORT", "3306")
	p, err := strconv.Atoi(mysqlPort)
	if err != nil || p < 1 || p > 65535 {
		return c, fmt.Errorf("MYSQL_PORT must be between 1 and 65535")
	}
	db := mysql.NewConfig()
	db.User = value("MYSQL_USER", "taskservice")
	db.Passwd = os.Getenv("MYSQL_PASSWORD")
	db.Net = "tcp"
	db.Addr = net.JoinHostPort(value("MYSQL_HOST", "localhost"), mysqlPort)
	db.DBName = value("MYSQL_DATABASE", "taskservice")
	db.ParseTime = true
	db.Loc = time.UTC
	db.Timeout = 5 * time.Second
	db.ReadTimeout = 10 * time.Second
	db.WriteTimeout = 10 * time.Second
	db.Params = map[string]string{"charset": "utf8mb4", "time_zone": "'+00:00'"}
	c.DSN = db.FormatDSN()
	return c, nil
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
