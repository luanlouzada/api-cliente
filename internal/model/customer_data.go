package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Create persiste um cliente já validado pelo Model e devolve os campos de
// auditoria definidos pelo PostgreSQL.
func (store *postgresData) Create(ctx context.Context, customer Customer) (Customer, error) {
	const query = `
		INSERT INTO customers (id, name, email, phone, password_hash, role)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, email, phone, role, created_at, updated_at
	`

	err := store.pool.QueryRow(
		ctx,
		query,
		customer.ID,
		customer.Name,
		customer.Email,
		customer.Phone,
		customer.PasswordHash,
		customer.Role,
	).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Email,
		&customer.Phone,
		&customer.Role,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if err != nil {
		return Customer{}, mapDatabaseError("criar cliente", err)
	}
	return customer, nil
}

// FindAll lista clientes em ordem determinística de identificador para que duas
// consultas ao mesmo estado produzam respostas previsíveis.
func (store *postgresData) FindAll(ctx context.Context) ([]Customer, error) {
	const query = `
		SELECT id, name, email, phone, role, created_at, updated_at
		FROM customers
		ORDER BY id
	`

	rows, err := store.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listar clientes: %w", err)
	}
	defer rows.Close()

	customers := make([]Customer, 0)
	for rows.Next() {
		var customer Customer
		if err := rows.Scan(
			&customer.ID,
			&customer.Name,
			&customer.Email,
			&customer.Phone,
			&customer.Role,
			&customer.CreatedAt,
			&customer.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("ler cliente: %w", err)
		}
		customers = append(customers, customer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("percorrer clientes: %w", err)
	}
	return customers, nil
}

// FindByID busca um cliente sem expor seu hash de senha e traduz ausência para
// o erro de negócio esperado pelo Controller.
func (store *postgresData) FindByID(ctx context.Context, id uuid.UUID) (Customer, error) {
	const query = `
		SELECT id, name, email, phone, role, created_at, updated_at
		FROM customers
		WHERE id = $1
	`

	var customer Customer
	err := store.pool.QueryRow(ctx, query, id).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Email,
		&customer.Phone,
		&customer.Role,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrCustomerNotFound
	}
	if err != nil {
		return Customer{}, fmt.Errorf("buscar cliente por id: %w", err)
	}
	return customer, nil
}

// Update altera somente os dados editáveis do perfil, preserva papel e senha e
// usa o relógio do banco para registrar a atualização.
func (store *postgresData) Update(
	ctx context.Context,
	id uuid.UUID,
	customer Customer,
) (Customer, error) {
	const query = `
		UPDATE customers
		SET name = $2,
			email = $3,
			phone = $4,
			updated_at = now()
		WHERE id = $1
		RETURNING id, name, email, phone, role, created_at, updated_at
	`

	err := store.pool.QueryRow(
		ctx,
		query,
		id,
		customer.Name,
		customer.Email,
		customer.Phone,
	).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Email,
		&customer.Phone,
		&customer.Role,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrCustomerNotFound
	}
	if err != nil {
		return Customer{}, mapDatabaseError("atualizar cliente", err)
	}
	return customer, nil
}

// Delete remove o cliente e deixa as chaves estrangeiras em cascata encerrarem
// suas sessões, distinguindo uma exclusão inexistente.
func (store *postgresData) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM customers WHERE id = $1`

	commandTag, err := store.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("excluir cliente: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrCustomerNotFound
	}
	return nil
}
