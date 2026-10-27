package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"taskservice/internal/auth"
	"taskservice/internal/cache"
	"taskservice/internal/config"
	"taskservice/internal/repository"
	"taskservice/internal/service"
	httpapi "taskservice/internal/transport/http"
	"taskservice/migrations"
)

func Run(ctx context.Context, c config.Config, log *slog.Logger) error {
	db, err := sql.Open("mysql", c.DSN)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	startup, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := db.PingContext(startup); err != nil {
		return fmt.Errorf("mysql ping: %w", err)
	}
	if err := migrations.Apply(startup, db); err != nil {
		return err
	}
	client := redis.NewClient(&redis.Options{Addr: c.RedisAddr, Password: c.RedisPassword, MaxRetries: -1, DialTimeout: time.Second, ReadTimeout: 250 * time.Millisecond, WriteTimeout: 250 * time.Millisecond, ContextTimeoutEnabled: true})
	defer client.Close()
	if err := client.Ping(startup).Err(); err != nil {
		log.Warn("redis unavailable; using database fallback", "error", err)
	}
	tokens := auth.New(c.JWTSecret, c.JWTTTL)
	svc := service.New(repository.New(db), tokens, cache.New(client, log))
	server := &http.Server{Addr: "0.0.0.0:" + c.HTTPPort, Handler: httpapi.New(svc, tokens, log, db.PingContext), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	log.Info("server started", "address", server.Addr)
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		log.Info("server shutting down")
		shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return fmt.Errorf("http shutdown: %w", err)
		}
		if err := <-done; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		log.Info("server stopped")
		return nil
	}
}
