package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	Port       string
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	Message    string
}

type App struct {
	db      *sql.DB
	message string
}

type Task struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type createTaskRequest struct {
	Title string `json:"title"`
}

type updateTaskRequest struct {
	Completed bool `json:"completed"`
}

type healthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func main() {
	cfg := loadConfig()

	db, err := openDatabase(cfg)
	if err != nil {
		log.Fatalf("database setup failed: %v", err)
	}
	defer db.Close()

	app := &App{db: db, message: cfg.Message}
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           requestLogger(app.routes()),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("taskboard API listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server stopped unexpectedly: %v", err)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("server stopped")
}

func loadConfig() Config {
	return Config{
		Port:       env("APP_PORT", "8080"),
		DBHost:     env("DB_HOST", "localhost"),
		DBPort:     env("DB_PORT", "5432"),
		DBName:     env("DB_NAME", "taskboard"),
		DBUser:     env("DB_USER", "taskboard"),
		DBPassword: env("DB_PASSWORD", "taskboard"),
		Message:    env("APP_MESSAGE", "running on Kubernetes"),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func openDatabase(cfg Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", a.live)
	mux.HandleFunc("GET /health/ready", a.ready)
	mux.HandleFunc("GET /api/tasks", a.listTasks)
	mux.HandleFunc("POST /api/tasks", a.createTask)
	mux.HandleFunc("PATCH /api/tasks/{id}", a.updateTask)
	mux.HandleFunc("DELETE /api/tasks/{id}", a.deleteTask)
	return mux
}

func (a *App) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Message: a.message})
}

func (a *App) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.db.PingContext(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, healthResponse{
			Status: "not-ready", Message: "database unavailable",
		})
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "ready", Message: a.message})
}

func (a *App) listTasks(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id, title, completed, created_at, updated_at
		FROM tasks
		ORDER BY created_at DESC`)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()

	tasks := make([]Task, 0)
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.Title, &task.Completed, &task.CreatedAt, &task.UpdatedAt); err != nil {
			serverError(w, err)
			return
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (a *App) createTask(w http.ResponseWriter, r *http.Request) {
	var input createTaskRequest
	if err := decodeJSON(w, r, &input); err != nil {
		clientError(w, http.StatusBadRequest, err.Error())
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		clientError(w, http.StatusUnprocessableEntity, "title is required")
		return
	}
	if len(input.Title) > 200 {
		clientError(w, http.StatusUnprocessableEntity, "title must be 200 characters or fewer")
		return
	}

	var task Task
	err := a.db.QueryRowContext(r.Context(), `
		INSERT INTO tasks (title)
		VALUES ($1)
		RETURNING id, title, completed, created_at, updated_at`, input.Title).
		Scan(&task.ID, &task.Title, &task.Completed, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (a *App) updateTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		clientError(w, http.StatusBadRequest, "invalid task id")
		return
	}

	var input updateTaskRequest
	if err := decodeJSON(w, r, &input); err != nil {
		clientError(w, http.StatusBadRequest, err.Error())
		return
	}

	var task Task
	err = a.db.QueryRowContext(r.Context(), `
		UPDATE tasks
		SET completed = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, title, completed, created_at, updated_at`, input.Completed, id).
		Scan(&task.ID, &task.Title, &task.Completed, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		clientError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (a *App) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		clientError(w, http.StatusBadRequest, "invalid task id")
		return
	}

	result, err := a.db.ExecContext(r.Context(), `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		serverError(w, err)
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		clientError(w, http.StatusNotFound, "task not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func clientError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("request failed: %v", err)
	clientError(w, http.StatusInternalServerError, "internal server error")
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s duration=%s", r.Method, r.URL.Path, time.Since(started).Round(time.Millisecond))
	})
}
