package config

import (
	"log"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	Database    DatabaseConfig
	Auth        AuthConfig
	Port        string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type AuthConfig struct {
	JWTSecret      string
	AccessTokenTTL time.Duration
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("arquivo .env nao encontrado, usando variaveis do ambiente")
	}

	database := DatabaseConfig{
		Host:     getEnv("POSTGRES_HOST", "localhost"),
		Port:     getEnv("POSTGRES_PORT", "5432"),
		User:     getEnv("POSTGRES_USER", "app"),
		Password: getEnv("POSTGRES_PASSWORD", "app"),
		Name:     getEnv("POSTGRES_DB", "app"),
		SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
	}

	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL == "" {
		databaseURL = database.URL()
	}

	return Config{
		DatabaseURL: databaseURL,
		Database:    database,
		Auth: AuthConfig{
			JWTSecret:      getEnv("JWT_SECRET", ""),
			AccessTokenTTL: getDurationEnv("JWT_ACCESS_TOKEN_TTL", time.Hour),
		},
		Port: getEnv("PORT", "8080"),
	}
}

func (database DatabaseConfig) URL() string {
	values := url.Values{}
	values.Set("sslmode", database.SSLMode)

	connectionURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(database.User, database.Password),
		Host:     net.JoinHostPort(database.Host, database.Port),
		Path:     "/" + database.Name,
		RawQuery: values.Encode(),
	}

	return connectionURL.String()
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("variavel %s invalida (%q), usando %s", key, value, fallback)
		return fallback
	}
	return duration
}
