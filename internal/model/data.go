package model

import "github.com/jackc/pgx/v5/pgxpool"

// postgresData é a parte do Model que lê e grava dados no PostgreSQL. Ela
// compartilha um pool de conexões seguro para uso concorrente.
type postgresData struct {
	pool *pgxpool.Pool
}

// newPostgresData associa o pool PostgreSQL às operações de dados do Model.
func newPostgresData(pool *pgxpool.Pool) *postgresData {
	return &postgresData{pool: pool}
}
