package model

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// mapDatabaseError converte restrições conhecidas do PostgreSQL em erros de
// negócio e preserva a causa original nas demais falhas de persistência. A
// operação deve nomear a ação concreta do banco; o Model adiciona depois o
// contexto mais amplo sem repetir a mesma expressão.
func mapDatabaseError(operation string, err error) error {
	var postgresError *pgconn.PgError
	// O PostgreSQL usa 23505 para violação de unicidade. Conferir também o nome
	// da restrição impede confundir e-mail duplicado com outra regra única.
	if errors.As(err, &postgresError) &&
		postgresError.Code == "23505" &&
		postgresError.ConstraintName == "customers_email_key" {
		return ErrCustomerEmailAlreadyExists
	}
	return fmt.Errorf("%s: %w", operation, err)
}
