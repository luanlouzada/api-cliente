package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// CreateWithRefreshToken grava cliente, família e primeiro token de renovação na
// mesma transação; a função transacional permite assinar o token de acesso antes
// da confirmação e faz toda a operação retroceder se a emissão falhar.
func (store *postgresData) CreateWithRefreshToken(
	ctx context.Context,
	customer Customer,
	token RefreshToken,
	beforeCommit func(Customer) error,
) (Customer, RefreshToken, error) {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("iniciar cadastro: %w", err)
	}
	defer rollbackTransaction(transaction)

	const customerQuery = `
		INSERT INTO customers (id, name, email, phone, password_hash, role)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, email, phone, role, created_at, updated_at
	`
	err = transaction.QueryRow(
		ctx,
		customerQuery,
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
		return Customer{}, RefreshToken{}, mapDatabaseError("cadastrar cliente", err)
	}

	token.CustomerID = customer.ID
	token, err = normalizeInitialRefreshToken(ctx, transaction, token)
	if err != nil {
		return Customer{}, RefreshToken{}, err
	}
	if err := insertRefreshTokenFamily(ctx, transaction, token); err != nil {
		return Customer{}, RefreshToken{}, err
	}
	if err := insertRefreshToken(ctx, transaction, token); err != nil {
		return Customer{}, RefreshToken{}, err
	}
	if beforeCommit == nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("cadastro sem função transacional")
	}
	if err := beforeCommit(customer); err != nil {
		return Customer{}, RefreshToken{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("confirmar cadastro: %w", err)
	}
	return customer, token, nil
}

// FindByEmail carrega o estado da credencial usado na primeira verificação do
// acesso; a criação da sessão confirmará esse estado sob bloqueio.
func (store *postgresData) FindByEmail(ctx context.Context, email string) (Customer, error) {
	const query = `
		SELECT id, name, email, phone, role, password_hash, created_at, updated_at
		FROM customers
		WHERE email = $1
	`

	var customer Customer
	err := store.pool.QueryRow(ctx, query, email).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Email,
		&customer.Phone,
		&customer.Role,
		&customer.PasswordHash,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrCustomerNotFound
	}
	if err != nil {
		return Customer{}, fmt.Errorf("buscar cliente por email: %w", err)
	}
	return customer, nil
}

// CreateRefreshToken cria uma nova família de sessão para um cliente existente,
// relendo o cliente sob bloqueio compartilhado antes da função transacional.
func (store *postgresData) CreateRefreshToken(
	ctx context.Context,
	token RefreshToken,
	beforeCommit func(Customer) error,
) (Customer, RefreshToken, error) {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("iniciar criação da sessão: %w", err)
	}
	defer rollbackTransaction(transaction)

	customer, err := lockRefreshCustomer(ctx, transaction, token.CustomerID)
	if err != nil {
		return Customer{}, RefreshToken{}, err
	}
	token, err = normalizeInitialRefreshToken(ctx, transaction, token)
	if err != nil {
		return Customer{}, RefreshToken{}, err
	}
	if beforeCommit == nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("criação de sessão sem função transacional")
	}
	if err := beforeCommit(customer); err != nil {
		return Customer{}, RefreshToken{}, err
	}
	if err := insertRefreshTokenFamily(ctx, transaction, token); err != nil {
		return Customer{}, RefreshToken{}, err
	}
	if err := insertRefreshToken(ctx, transaction, token); err != nil {
		return Customer{}, RefreshToken{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("confirmar criação da sessão: %w", err)
	}
	return customer, token, nil
}

// RotateRefreshToken serializa a família, detecta expiração ou reuso e troca o
// token ativo de modo atômico, respeitando a ordem cliente, família e token dos
// bloqueios para evitar corridas e impasses.
func (store *postgresData) RotateRefreshToken(
	ctx context.Context,
	currentHash []byte,
	replacement RefreshToken,
	beforeCommit func(Customer) error,
) (Customer, RefreshToken, error) {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("iniciar rotação: %w", err)
	}
	defer rollbackTransaction(transaction)

	// A primeira consulta apenas descobre os identificadores. Em seguida, todas as
	// operações de sessão bloqueiam cliente, família e token sempre nessa ordem.
	// Isso impede um impasse no qual duas transações esperariam uma pela outra.
	familyID, customerID, err := findRefreshTokenFamily(ctx, transaction, currentHash)
	if err != nil {
		return Customer{}, RefreshToken{}, err
	}
	customer, err := lockRefreshCustomer(ctx, transaction, customerID)
	if err != nil {
		return Customer{}, RefreshToken{}, err
	}

	familyExpiresAt, familyRevokedAt, err := lockRefreshTokenFamily(ctx, transaction, familyID)
	if err != nil {
		return Customer{}, RefreshToken{}, err
	}
	if familyRevokedAt != nil {
		return Customer{}, RefreshToken{}, ErrRefreshTokenInvalid
	}

	const tokenQuery = `
		SELECT
			rt.expires_at,
			rt.revoked_at
		FROM refresh_tokens AS rt
		WHERE rt.token_hash = $1 AND rt.family_id = $2
		FOR UPDATE OF rt
	`

	var (
		expiresAt time.Time
		revokedAt *time.Time
	)
	err = transaction.QueryRow(ctx, tokenQuery, currentHash, familyID).Scan(
		&expiresAt,
		&revokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, RefreshToken{}, ErrRefreshTokenInvalid
	}
	if err != nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("buscar refresh token: %w", err)
	}
	// O relógio é consultado depois dos bloqueios. Se esta transação esperou outra
	// rotação terminar, a validade será comparada com o instante atual do banco,
	// não com um horário antigo capturado antes da espera.
	databaseNow, err := currentDatabaseTime(ctx, transaction)
	if err != nil {
		return Customer{}, RefreshToken{}, err
	}

	idleTTL := replacement.ExpiresAt.Sub(replacement.CreatedAt)
	if replacement.CreatedAt.IsZero() || idleTTL <= 0 {
		return Customer{}, RefreshToken{}, fmt.Errorf("refresh token substituto sem data de criação")
	}
	now := databaseNow.UTC()
	if revokedAt != nil {
		// Reutilizar um token antigo é evidência de possível cópia da credencial.
		// A revogação precisa ser confirmada mesmo que a resposta final seja erro.
		// Sem confirmar a transação, a reversão automática desfaria justamente a
		// proteção contra reuso.
		if err := revokeRefreshTokenFamily(ctx, transaction, familyID, now); err != nil {
			return Customer{}, RefreshToken{}, err
		}
		if err := transaction.Commit(ctx); err != nil {
			return Customer{}, RefreshToken{}, fmt.Errorf("confirmar revogação por reutilização: %w", err)
		}
		return Customer{}, RefreshToken{}, ErrRefreshTokenReused
	}

	if !expiresAt.After(now) || !familyExpiresAt.After(now) {
		// A expiração também é persistida como revogação para encerrar a família de
		// forma inequívoca e manter uma trilha de auditoria no banco.
		if err := revokeRefreshTokenFamily(ctx, transaction, familyID, now); err != nil {
			return Customer{}, RefreshToken{}, err
		}
		if err := transaction.Commit(ctx); err != nil {
			return Customer{}, RefreshToken{}, fmt.Errorf("confirmar revogação por expiração: %w", err)
		}
		return Customer{}, RefreshToken{}, ErrRefreshTokenExpired
	}

	replacement.CustomerID = customerID
	replacement.FamilyID = familyID
	replacement.FamilyExpiresAt = familyExpiresAt
	replacement.CreatedAt = now
	replacement.ExpiresAt = now.Add(idleTTL)
	if replacement.ExpiresAt.After(familyExpiresAt) {
		replacement.ExpiresAt = familyExpiresAt
	}
	if beforeCommit == nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("rotação sem função transacional")
	}
	if err := beforeCommit(customer); err != nil {
		return Customer{}, RefreshToken{}, err
	}

	// Revogar o token atual, inserir o substituto e ligá-los ocorre na mesma
	// transação. Portanto, concorrentes nunca observam uma rotação pela metade.
	const revokeQuery = `
		UPDATE refresh_tokens
		SET revoked_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL
	`
	commandTag, err := transaction.Exec(ctx, revokeQuery, currentHash, now)
	if err != nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("revogar refresh token anterior: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return Customer{}, RefreshToken{}, ErrRefreshTokenReused
	}
	if err := insertRefreshToken(ctx, transaction, replacement); err != nil {
		return Customer{}, RefreshToken{}, err
	}
	const linkReplacementQuery = `
		UPDATE refresh_tokens
		SET replaced_by = $2
		WHERE token_hash = $1 AND revoked_at IS NOT NULL AND replaced_by IS NULL
	`
	commandTag, err = transaction.Exec(ctx, linkReplacementQuery, currentHash, replacement.ID)
	if err != nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("vincular refresh token substituto: %w", err)
	}
	if commandTag.RowsAffected() != 1 {
		return Customer{}, RefreshToken{}, ErrRefreshTokenReused
	}
	if err := transaction.Commit(ctx); err != nil {
		return Customer{}, RefreshToken{}, fmt.Errorf("confirmar rotação: %w", err)
	}
	return customer, replacement, nil
}

// RevokeRefreshToken encerra de forma idempotente toda a família associada ao
// hash apresentado, usando a mesma ordem de bloqueios adotada pela rotação.
func (store *postgresData) RevokeRefreshToken(ctx context.Context, tokenHash []byte) error {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar revogação da sessão: %w", err)
	}
	defer rollbackTransaction(transaction)

	familyID, customerID, err := findRefreshTokenFamily(ctx, transaction, tokenHash)
	if errors.Is(err, ErrRefreshTokenInvalid) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := lockRefreshCustomer(ctx, transaction, customerID); err != nil {
		if errors.Is(err, ErrRefreshTokenInvalid) {
			return nil
		}
		return err
	}
	_, _, err = lockRefreshTokenFamily(ctx, transaction, familyID)
	if err != nil {
		if errors.Is(err, ErrRefreshTokenInvalid) {
			return nil
		}
		return err
	}
	databaseNow, err := currentDatabaseTime(ctx, transaction)
	if err != nil {
		return err
	}
	if err := revokeRefreshTokenFamily(ctx, transaction, familyID, databaseNow.UTC()); err != nil {
		return err
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("confirmar revogação da sessão: %w", err)
	}
	return nil
}

// findRefreshTokenFamily resolve família e cliente antes dos bloqueios; as
// funções seguintes validam novamente o estado dentro da transação.
func findRefreshTokenFamily(
	ctx context.Context,
	transaction pgx.Tx,
	tokenHash []byte,
) (uuid.UUID, uuid.UUID, error) {
	const query = `
		SELECT token.family_id, family.customer_id
		FROM refresh_tokens AS token
		JOIN refresh_token_families AS family ON family.id = token.family_id
		WHERE token.token_hash = $1
	`

	var familyID uuid.UUID
	var customerID uuid.UUID
	if err := transaction.QueryRow(ctx, query, tokenHash).Scan(&familyID, &customerID); errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, ErrRefreshTokenInvalid
	} else if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("buscar família do refresh token: %w", err)
	}
	return familyID, customerID, nil
}

// lockRefreshCustomer relê o cliente com FOR SHARE. Esse bloqueio compartilhado
// permite outras leituras, mas impede alteração ou exclusão da linha até o fim
// da transação, estabilizando perfil, papel e credencial durante a sessão.
func lockRefreshCustomer(
	ctx context.Context,
	transaction pgx.Tx,
	customerID uuid.UUID,
) (Customer, error) {
	const query = `
		SELECT id, name, email, phone, role, password_hash, created_at, updated_at
		FROM customers
		WHERE id = $1
		FOR SHARE
	`

	var customer Customer
	err := transaction.QueryRow(ctx, query, customerID).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Email,
		&customer.Phone,
		&customer.Role,
		&customer.PasswordHash,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrRefreshTokenInvalid
	} else if err != nil {
		return Customer{}, fmt.Errorf("bloquear cliente da sessão: %w", err)
	}
	return customer, nil
}

// lockRefreshTokenFamily usa FOR UPDATE para dar à transação acesso exclusivo à
// família. Renovações, logout e detecção de reuso da mesma sessão passam por esse
// ponto uma de cada vez.
func lockRefreshTokenFamily(
	ctx context.Context,
	transaction pgx.Tx,
	familyID uuid.UUID,
) (time.Time, *time.Time, error) {
	const query = `
		SELECT expires_at, revoked_at
		FROM refresh_token_families
		WHERE id = $1
		FOR UPDATE
	`

	var expiresAt time.Time
	var revokedAt *time.Time
	if err := transaction.QueryRow(ctx, query, familyID).Scan(&expiresAt, &revokedAt); errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil, ErrRefreshTokenInvalid
	} else if err != nil {
		return time.Time{}, nil, fmt.Errorf("bloquear família do refresh token: %w", err)
	}
	return expiresAt, revokedAt, nil
}

// currentDatabaseTime consulta clock_timestamp, função que devolve o horário real
// do PostgreSQL naquele ponto da transação. A validade não é calculada com um
// instante capturado antes de uma espera longa por bloqueios.
func currentDatabaseTime(ctx context.Context, transaction pgx.Tx) (time.Time, error) {
	var databaseNow time.Time
	if err := transaction.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&databaseNow); err != nil {
		return time.Time{}, fmt.Errorf("consultar relógio do banco: %w", err)
	}
	return databaseNow, nil
}

// rollbackTransaction pede ao PostgreSQL para desfazer uma transação que não foi
// confirmada. Usa um contexto independente mesmo se a requisição já expirou; o
// pgx permite chamar Rollback sem efeito depois de uma confirmação bem-sucedida.
func rollbackTransaction(transaction pgx.Tx) {
	rollbackContext, cancelRollback := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelRollback()
	_ = transaction.Rollback(rollbackContext)
}

// normalizeInitialRefreshToken preserva as durações definidas pelo gerenciador
// de tokens, mas ancora as datas no relógio do PostgreSQL. Usar um único relógio
// evita decisões diferentes quando aplicação e banco estão alguns segundos fora.
func normalizeInitialRefreshToken(
	ctx context.Context,
	transaction pgx.Tx,
	token RefreshToken,
) (RefreshToken, error) {
	idleTTL := token.ExpiresAt.Sub(token.CreatedAt)
	absoluteTTL := token.FamilyExpiresAt.Sub(token.CreatedAt)
	if token.CreatedAt.IsZero() || idleTTL <= 0 || absoluteTTL < idleTTL {
		return RefreshToken{}, fmt.Errorf("refresh token inicial com prazos inválidos")
	}

	databaseNow, err := currentDatabaseTime(ctx, transaction)
	if err != nil {
		return RefreshToken{}, err
	}
	token.CreatedAt = databaseNow.UTC()
	token.ExpiresAt = token.CreatedAt.Add(idleTTL)
	token.FamilyExpiresAt = token.CreatedAt.Add(absoluteTTL)
	return token, nil
}

// refreshTokenExecutor descreve somente a operação SQL usada pelos auxiliares
// abaixo. Assim eles funcionam com qualquer execução pertencente à transação.
type refreshTokenExecutor interface {
	// Exec envia ao PostgreSQL um comando que não devolve linhas de dados.
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// insertRefreshTokenFamily persiste o limite absoluto e o dono da família antes
// de qualquer token que dependa dessa linha.
func insertRefreshTokenFamily(
	ctx context.Context,
	executor refreshTokenExecutor,
	token RefreshToken,
) error {
	const query = `
		INSERT INTO refresh_token_families (
			id, customer_id, expires_at, created_at
		)
		VALUES ($1, $2, $3, $4)
	`
	if _, err := executor.Exec(
		ctx,
		query,
		token.FamilyID,
		token.CustomerID,
		token.FamilyExpiresAt,
		token.CreatedAt,
	); err != nil {
		return fmt.Errorf("inserir família do refresh token: %w", err)
	}
	return nil
}

// insertRefreshToken grava somente o hash do segredo e vincula o token a uma
// família cuja expiração já foi estabilizada.
func insertRefreshToken(
	ctx context.Context,
	executor refreshTokenExecutor,
	token RefreshToken,
) error {
	const query = `
		INSERT INTO refresh_tokens (
			id, family_id, token_hash, expires_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`
	if _, err := executor.Exec(
		ctx,
		query,
		token.ID,
		token.FamilyID,
		token.TokenHash,
		token.ExpiresAt,
		token.CreatedAt,
	); err != nil {
		return fmt.Errorf("inserir refresh token: %w", err)
	}
	return nil
}

// revokeRefreshTokenFamily marca a família e todos os seus tokens sem substituir
// datas de revogação anteriores, preservando a trilha de auditoria.
func revokeRefreshTokenFamily(
	ctx context.Context,
	executor refreshTokenExecutor,
	familyID uuid.UUID,
	revokedAt time.Time,
) error {
	const familyQuery = `
		UPDATE refresh_token_families
		SET revoked_at = COALESCE(revoked_at, $2)
		WHERE id = $1
	`
	if _, err := executor.Exec(ctx, familyQuery, familyID, revokedAt); err != nil {
		return fmt.Errorf("revogar família do refresh token: %w", err)
	}

	const tokensQuery = `
		UPDATE refresh_tokens
		SET revoked_at = COALESCE(revoked_at, $2)
		WHERE family_id = $1
	`
	if _, err := executor.Exec(ctx, tokensQuery, familyID, revokedAt); err != nil {
		return fmt.Errorf("revogar refresh tokens da família: %w", err)
	}
	return nil
}
