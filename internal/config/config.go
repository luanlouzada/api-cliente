package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config reúne endereços, segredos e limites validados antes de iniciar a API.
type Config struct {
	HTTPHost    string
	Port        string
	DatabaseURL string
	Auth        AuthConfig
	Server      ServerConfig
}

// AuthConfig define segredos e prazos usados pelos mecanismos de autenticação.
type AuthConfig struct {
	JWTSecret               string
	AccessTokenTTL          time.Duration
	RefreshTokenIdleTTL     time.Duration
	RefreshTokenAbsoluteTTL time.Duration
}

// ServerConfig concentra os prazos de rede e encerramento do servidor HTTP.
type ServerConfig struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// databaseConfig reúne os componentes usados somente para montar uma URL de
// conexão quando DATABASE_URL não foi informada diretamente.
type databaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// Load combina .env, variáveis de ambiente e valores padrão, validando o
// resultado antes que qualquer dependência externa seja inicializada.
func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	accessTokenTTL, err := durationFromEnv("JWT_ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	refreshTokenIdleTTL, err := durationFromEnv("REFRESH_TOKEN_IDLE_TTL", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	refreshTokenAbsoluteTTL, err := durationFromEnv("REFRESH_TOKEN_ABSOLUTE_TTL", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		HTTPHost:    envOrDefault("HTTP_HOST", "127.0.0.1"),
		Port:        envOrDefault("PORT", "8080"),
		DatabaseURL: databaseURLFromEnvironment(),
		Auth: AuthConfig{
			JWTSecret:               os.Getenv("JWT_SECRET"),
			AccessTokenTTL:          accessTokenTTL,
			RefreshTokenIdleTTL:     refreshTokenIdleTTL,
			RefreshTokenAbsoluteTTL: refreshTokenAbsoluteTTL,
		},
		Server: ServerConfig{
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
			ShutdownTimeout:   10 * time.Second,
		},
	}

	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

// LoadDatabaseURL expõe a mesma URL de conexão, com suporte a .env, usada pela
// aplicação para impedir divergências entre ferramentas e o processo da API.
func LoadDatabaseURL() (string, error) {
	if err := loadDotEnv(); err != nil {
		return "", err
	}
	return databaseURLFromEnvironment(), nil
}

// LoadFrontendURL lê somente as opções HTTP para que ferramentas locais possam
// mostrar o endereço do frontend sem exigir segredos de banco ou autenticação.
func LoadFrontendURL() (string, error) {
	if err := loadDotEnv(); err != nil {
		return "", err
	}
	host := envOrDefault("HTTP_HOST", "127.0.0.1")
	if err := validateHTTPHost(host); err != nil {
		return "", err
	}
	port := envOrDefault("PORT", "8080")
	if err := validatePort(port); err != nil {
		return "", err
	}
	if host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port) + "/", nil
}

// Validate verifica os invariantes da configuração completa antes da inicialização,
// incluindo limites de porta, segredos e durações relacionadas entre si.
func (config Config) Validate() error {
	if err := validateHTTPHost(config.HTTPHost); err != nil {
		return err
	}
	if err := validatePort(config.Port); err != nil {
		return err
	}
	// As mensagens destas invariantes são fixas e não envolvem outra causa.
	// errors.New comunica essa intenção sem oferecer formatação desnecessária.
	if config.DatabaseURL == "" {
		return errors.New("DATABASE_URL não pode ser vazia")
	}
	if len(config.Auth.JWTSecret) < 32 {
		return errors.New("JWT_SECRET deve ter pelo menos 32 bytes")
	}
	if config.Auth.AccessTokenTTL < 10*time.Second {
		return errors.New("JWT_ACCESS_TOKEN_TTL deve ser de pelo menos 10 segundos")
	}
	if config.Auth.RefreshTokenIdleTTL <= 0 {
		return errors.New("REFRESH_TOKEN_IDLE_TTL deve ser positivo")
	}
	if config.Auth.RefreshTokenAbsoluteTTL < config.Auth.RefreshTokenIdleTTL {
		return errors.New(
			"REFRESH_TOKEN_ABSOLUTE_TTL deve ser maior ou igual a REFRESH_TOKEN_IDLE_TTL",
		)
	}
	return nil
}

// validateHTTPHost restringe a escuta a localhost ou a um IP explícito, evitando
// que nomes ambíguos sejam aceitos como endereço de rede.
func validateHTTPHost(value string) error {
	if value == "localhost" || net.ParseIP(value) != nil {
		return nil
	}
	return errors.New("HTTP_HOST deve ser localhost ou um endereço IP")
}

// validatePort exige uma porta TCP composta somente por dígitos ASCII e dentro
// do intervalo permitido, sem sinais ou espaços aceitos por conversores gerais.
func validatePort(value string) error {
	if value == "" {
		return errors.New("PORT deve ser um número entre 1 e 65535")
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return errors.New("PORT deve ser um número entre 1 e 65535")
		}
	}

	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("PORT deve ser um número entre 1 e 65535")
	}
	return nil
}

// URL monta a URL de conexão PostgreSQL codificando usuário, senha, host e nome
// do banco, em vez de concatenar valores que podem conter caracteres especiais.
func (database databaseConfig) URL() string {
	query := url.Values{}
	query.Set("sslmode", database.SSLMode)

	connectionURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(database.User, database.Password),
		Host:     net.JoinHostPort(database.Host, database.Port),
		Path:     "/" + database.Name,
		RawPath:  "/" + url.PathEscape(database.Name),
		RawQuery: query.Encode(),
	}
	return connectionURL.String()
}

// databaseURLFromEnvironment prioriza DATABASE_URL e, na ausência dela,
// compõe a conexão a partir das variáveis POSTGRES_*.
func databaseURLFromEnvironment() string {
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		return databaseURL
	}
	return databaseConfig{
		Host:     envOrDefault("POSTGRES_HOST", "localhost"),
		Port:     envOrDefault("POSTGRES_PORT", "5432"),
		User:     envOrDefault("POSTGRES_USER", "app"),
		Password: envOrDefault("POSTGRES_PASSWORD", "app"),
		Name:     envOrDefault("POSTGRES_DB", "app"),
		SSLMode:  envOrDefault("POSTGRES_SSLMODE", "disable"),
	}.URL()
}

// loadDotEnv carrega o arquivo .env quando presente e trata sua ausência como
// um caso normal para ambientes configurados diretamente pelo processo.
func loadDotEnv() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("carregar .env: %w", err)
	}
	return nil
}

// envOrDefault devolve o valor não vazio do ambiente ou o padrão informado.
func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// durationFromEnv interpreta durações aceitas por time.ParseDuration, como
// "15m" para quinze minutos ou "24h" para vinte e quatro horas. Quando a
// variável não existe, preserva o valor padrão recebido.
func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s inválida (%q): %w", key, value, err)
	}
	return duration, nil
}
