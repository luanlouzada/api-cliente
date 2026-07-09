package repository

import (
	"context"
	"errors"
	"fmt"

	"cliente-api/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerRepository struct {
	pool *pgxpool.Pool
}

func NewCustomerRepository(pool *pgxpool.Pool) *CustomerRepository {
	return &CustomerRepository{pool: pool}
}

func (repo *CustomerRepository) Create(ctx context.Context, customer model.Customer) (model.Customer, error) {
	const query = `
		INSERT INTO customers (name, email, phone, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, email, phone, created_at, updated_at
	`

	err := repo.pool.QueryRow(
		ctx,
		query,
		customer.Name,
		customer.Email,
		customer.Phone,
		customer.PasswordHash,
	).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Email,
		&customer.Phone,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if err != nil {
		return model.Customer{}, mapDatabaseError("create customer", err)
	}

	return customer, nil
}

func (repo *CustomerRepository) FindAll(ctx context.Context) ([]model.Customer, error) {
	const query = `
		SELECT id, name, email, phone, created_at, updated_at
		FROM customers
		ORDER BY id
	`

	rows, err := repo.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("find all customers: %w", err)
	}
	defer rows.Close()

	customers := make([]model.Customer, 0)
	for rows.Next() {
		var customer model.Customer
		err = rows.Scan(
			&customer.ID,
			&customer.Name,
			&customer.Email,
			&customer.Phone,
			&customer.CreatedAt,
			&customer.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan customer: %w", err)
		}

		customers = append(customers, customer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate customers: %w", err)
	}

	return customers, nil
}

func (repo *CustomerRepository) FindByID(ctx context.Context, id string) (model.Customer, error) {
	const query = `
		SELECT id, name, email, phone, created_at, updated_at
		FROM customers
		WHERE id = $1::uuid
	`

	var customer model.Customer
	err := repo.pool.QueryRow(ctx, query, id).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Email,
		&customer.Phone,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Customer{}, model.ErrCustomerNotFound
	}
	if err != nil {
		return model.Customer{}, fmt.Errorf("find customer by id: %w", err)
	}
	return customer, nil
}

func (repo *CustomerRepository) FindByEmail(ctx context.Context, email string) (model.Customer, error) {
	const query = `
		SELECT id, name, email, phone, password_hash, created_at, updated_at
		FROM customers
		WHERE email = $1
	`

	var customer model.Customer
	err := repo.pool.QueryRow(ctx, query, email).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Email,
		&customer.Phone,
		&customer.PasswordHash,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Customer{}, model.ErrCustomerNotFound
	}
	if err != nil {
		return model.Customer{}, fmt.Errorf("find customer by email: %w", err)
	}

	return customer, nil
}

func (repo *CustomerRepository) Update(ctx context.Context, id string, customer model.Customer) (model.Customer, error) {
	const query = `
		UPDATE customers
		SET name = $2,
			email = $3,
			phone = $4,
			updated_at = now()
		WHERE id = $1::uuid
		RETURNING id, name, email, phone, created_at, updated_at
	`

	err := repo.pool.QueryRow(
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
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Customer{}, model.ErrCustomerNotFound
	}
	if err != nil {
		return model.Customer{}, mapDatabaseError("update customer", err)
	}

	return customer, nil
}

func (repo *CustomerRepository) Delete(ctx context.Context, id string) error {
	const query = `
		DELETE FROM customers
		WHERE id = $1::uuid
	`

	commandTag, err := repo.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete customer: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return model.ErrCustomerNotFound
	}
	return nil
}

func mapDatabaseError(operation string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return model.ErrCustomerEmailAlreadyExists
	}
	return fmt.Errorf("%s: %w", operation, err)
}
