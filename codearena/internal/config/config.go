// Package config loads application configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for both the server and the worker.
type Config struct {
	Port          string
	DatabaseURL   string
	KafkaBrokers  []string
	JWTSecret     string
	Executor      string // "local" or "k8s"
	K8sNamespace  string
	RunnerImage   string
	MigrationsDir string
	RunTimeoutMS  int // hard wall-clock limit for one run
}

// Load reads configuration from the environment, applying dev-friendly defaults.
func Load() Config {
	return Config{
		Port:          getenv("PORT", "8080"),
		DatabaseURL:   getenv("DATABASE_URL", "postgres://codearena:codearena@localhost:5432/codearena?sslmode=disable"),
		KafkaBrokers:  splitCSV(getenv("KAFKA_BROKERS", "localhost:9092")),
		JWTSecret:     getenv("JWT_SECRET", "dev-secret-change-me"),
		Executor:      getenv("EXECUTOR", "local"),
		K8sNamespace:  getenv("K8S_NAMESPACE", "codearena"),
		RunnerImage:   getenv("RUNNER_IMAGE", "codearena-runner:latest"),
		MigrationsDir: getenv("MIGRATIONS_DIR", "./migrations"),
		RunTimeoutMS:  getenvInt("RUN_TIMEOUT_MS", 10000),
	}
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
