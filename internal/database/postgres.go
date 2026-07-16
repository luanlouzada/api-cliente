package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool interpreta a URL, aplica limites conservadores ao conjunto e
// confirma a conectividade antes de devolver uma dependência pronta para uso.
func NewPostgresPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("interpretar configuração do postgres: %w", err)
	}

	// Um pool mantém e reutiliza várias conexões, permitindo requisições
	// concorrentes sem abrir uma conexão TCP a cada consulta. Estes limites
	// controlam o consumo de conexões e reciclam conexões antigas ou ociosas.
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnIdleTime = 10 * time.Minute
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("criar pool do postgres: %w", err)
	}
	// Criar o pool não garante, por si só, que o banco esteja acessível. Ping faz
	// essa verificação ainda na inicialização, antes de a API aceitar tráfego.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("conectar ao postgres: %w", err)
	}
	return pool, nil
}
