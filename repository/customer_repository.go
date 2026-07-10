package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cliente-api/model"

	"github.com/google/uuid"
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

func (repo *CustomerRepository) CreateWithRefreshToken(
	ctx context.Context,
	customer model.Customer,
	token model.RefreshToken,
) (model.Customer, error) {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return model.Customer{}, fmt.Errorf("begin customer registration: %w", err)
	}
	defer tx.Rollback(ctx)

	const customerQuery = `
		INSERT INTO customers (name, email, phone, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, email, phone, created_at, updated_at
	`
	err = tx.QueryRow(
		ctx,
		customerQuery,
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

	token.CustomerID = customer.ID
	const tokenQuery = `
		INSERT INTO refresh_tokens (
			id, customer_id, family_id, token_hash, expires_at, family_expires_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if _, err := tx.Exec(
		ctx,
		tokenQuery,
		token.ID,
		token.CustomerID,
		token.FamilyID,
		token.TokenHash,
		token.ExpiresAt,
		token.FamilyExpiresAt,
		token.CreatedAt,
	); err != nil {
		return model.Customer{}, fmt.Errorf("create initial refresh token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Customer{}, fmt.Errorf("commit customer registration: %w", err)
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

func (repo *CustomerRepository) CreateRefreshToken(ctx context.Context, token model.RefreshToken) error {
	const query = `
		INSERT INTO refresh_tokens (
			id, customer_id, family_id, token_hash, expires_at, family_expires_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := repo.pool.Exec(
		ctx,
		query,
		token.ID,
		token.CustomerID,
		token.FamilyID,
		token.TokenHash,
		token.ExpiresAt,
		token.FamilyExpiresAt,
		token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}

	return nil
}

func (repo *CustomerRepository) RotateRefreshToken(
	ctx context.Context,
	currentHash []byte,
	replacement model.RefreshToken,
) (model.Customer, model.RefreshToken, error) {
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return model.Customer{}, model.RefreshToken{}, fmt.Errorf("begin refresh token rotation: %w", err)
	}
	defer tx.Rollback(ctx)

	const findQuery = `
		SELECT
			rt.customer_id,
			rt.family_id,
			rt.expires_at,
			rt.family_expires_at,
			rt.revoked_at,
			c.id,
			c.name,
			c.email,
			c.phone,
			c.created_at,
			c.updated_at
		FROM refresh_tokens AS rt
		JOIN customers AS c ON c.id = rt.customer_id
		WHERE rt.token_hash = $1
		FOR UPDATE OF rt
	`

	var (
		customerID      uuid.UUID
		familyID        uuid.UUID
		expiresAt       time.Time
		familyExpiresAt time.Time
		revokedAt       *time.Time
		customer        model.Customer
	)
	err = tx.QueryRow(ctx, findQuery, currentHash).Scan(
		&customerID,
		&familyID,
		&expiresAt,
		&familyExpiresAt,
		&revokedAt,
		&customer.ID,
		&customer.Name,
		&customer.Email,
		&customer.Phone,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Customer{}, model.RefreshToken{}, model.ErrRefreshTokenInvalid
	}
	if err != nil {
		return model.Customer{}, model.RefreshToken{}, fmt.Errorf("find refresh token: %w", err)
	}

	now := time.Now().UTC()
	if revokedAt != nil {
		if err := revokeRefreshTokenFamily(ctx, tx, familyID, now); err != nil {
			return model.Customer{}, model.RefreshToken{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return model.Customer{}, model.RefreshToken{}, fmt.Errorf("commit refresh token reuse revocation: %w", err)
		}
		return model.Customer{}, model.RefreshToken{}, model.ErrRefreshTokenReused
	}

	if !expiresAt.After(now) || !familyExpiresAt.After(now) {
		if err := revokeRefreshTokenFamily(ctx, tx, familyID, now); err != nil {
			return model.Customer{}, model.RefreshToken{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return model.Customer{}, model.RefreshToken{}, fmt.Errorf("commit expired refresh token family revocation: %w", err)
		}
		return model.Customer{}, model.RefreshToken{}, model.ErrRefreshTokenExpired
	}

	replacement.CustomerID = customerID
	replacement.FamilyID = familyID
	replacement.FamilyExpiresAt = familyExpiresAt
	if replacement.ExpiresAt.After(familyExpiresAt) {
		replacement.ExpiresAt = familyExpiresAt
	}
	const insertQuery = `
		INSERT INTO refresh_tokens (
			id, customer_id, family_id, token_hash, expires_at, family_expires_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if _, err := tx.Exec(
		ctx,
		insertQuery,
		replacement.ID,
		replacement.CustomerID,
		replacement.FamilyID,
		replacement.TokenHash,
		replacement.ExpiresAt,
		replacement.FamilyExpiresAt,
		replacement.CreatedAt,
	); err != nil {
		return model.Customer{}, model.RefreshToken{}, fmt.Errorf("insert replacement refresh token: %w", err)
	}

	const revokeQuery = `
		UPDATE refresh_tokens
		SET revoked_at = $2, replaced_by = $3
		WHERE token_hash = $1 AND revoked_at IS NULL
	`
	commandTag, err := tx.Exec(ctx, revokeQuery, currentHash, now, replacement.ID)
	if err != nil {
		return model.Customer{}, model.RefreshToken{}, fmt.Errorf("rotate refresh token: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return model.Customer{}, model.RefreshToken{}, model.ErrRefreshTokenReused
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Customer{}, model.RefreshToken{}, fmt.Errorf("commit refresh token rotation: %w", err)
	}

	return customer, replacement, nil
}

func (repo *CustomerRepository) RevokeRefreshToken(ctx context.Context, tokenHash []byte) error {
	const query = `
		UPDATE refresh_tokens
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE family_id = (
			SELECT family_id
			FROM refresh_tokens
			WHERE token_hash = $1
		)
	`

	if _, err := repo.pool.Exec(ctx, query, tokenHash); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	return nil
}

type refreshTokenTx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func revokeRefreshTokenFamily(ctx context.Context, tx refreshTokenTx, familyID uuid.UUID, revokedAt time.Time) error {
	const query = `
		UPDATE refresh_tokens
		SET revoked_at = COALESCE(revoked_at, $2)
		WHERE family_id = $1
	`

	if _, err := tx.Exec(ctx, query, familyID, revokedAt); err != nil {
		return fmt.Errorf("revoke refresh token family: %w", err)
	}
	return nil
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
