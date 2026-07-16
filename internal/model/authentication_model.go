package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterInput reutiliza o contrato de criação porque o cadastro público pede
// os mesmos campos, embora siga regras de autorização diferentes.
type RegisterInput = CreateCustomerInput

// LoginInput contém as credenciais recebidas pela operação de login do Model.
type LoginInput struct {
	Email    string
	Password string
}

// AuthenticationResult reúne a sessão emitida e o cliente autenticado. Mapper e
// View decidem depois como representar esses dados no formato HTTP da API.
type AuthenticationResult struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	SessionExpiresAt time.Time
	Customer         Customer
}

// AuthenticationModel reúne as regras e os dados de cadastro, login, renovação
// e logout. Como parte do Model MVC, ele não conhece requisições HTTP.
type AuthenticationModel struct {
	records            authenticationRecords
	passwordHasher     passwordProtector
	accessTokenIssuer  accessTokenGenerator
	refreshTokenIssuer refreshTokenGenerator
}

// NewAuthenticationModel cria a parte do Model responsável pela autenticação
// usando o PostgreSQL e os mecanismos criptográficos configurados.
func NewAuthenticationModel(
	pool *pgxpool.Pool,
	passwordHasher BcryptPasswordHasher,
	accessTokenIssuer *AccessTokenManager,
	refreshTokenIssuer *RefreshTokenManager,
) *AuthenticationModel {
	return newAuthenticationModel(
		newPostgresData(pool),
		passwordHasher,
		accessTokenIssuer,
		refreshTokenIssuer,
	)
}

// newAuthenticationModel monta a estrutura a partir dos componentes internos
// usados pelas regras de autenticação.
func newAuthenticationModel(
	records authenticationRecords,
	passwordHasher passwordProtector,
	accessTokenIssuer accessTokenGenerator,
	refreshTokenIssuer refreshTokenGenerator,
) *AuthenticationModel {
	return &AuthenticationModel{
		records:            records,
		passwordHasher:     passwordHasher,
		accessTokenIssuer:  accessTokenIssuer,
		refreshTokenIssuer: refreshTokenIssuer,
	}
}

// Register valida e protege os dados, cria cliente e sessão inicial numa única
// transação e emite o token de acesso para o estado efetivamente persistido.
// Retorna a sessão completa ou um erro sem deixar cadastro parcial no banco.
func (model *AuthenticationModel) Register(
	ctx context.Context,
	input RegisterInput,
) (AuthenticationResult, error) {
	customer, err := buildCustomer(input, model.passwordHasher)
	if err != nil {
		return AuthenticationResult{}, err
	}

	rawRefreshToken, refreshToken, err := model.newRefreshToken(customer.ID)
	if err != nil {
		return AuthenticationResult{}, err
	}

	// Esta função é executada dentro da transação do banco. Se a assinatura falhar,
	// a transação não é confirmada e cliente, família e token são desfeitos juntos.
	var accessToken string
	var accessExpiresAt time.Time
	created, refreshToken, err := model.records.CreateWithRefreshToken(
		ctx,
		customer,
		refreshToken,
		func(persistedCustomer Customer) error {
			var issueErr error
			accessToken, accessExpiresAt, issueErr = model.accessTokenIssuer.Generate(persistedCustomer)
			if issueErr != nil {
				return fmt.Errorf("emitir token de acesso: %w", issueErr)
			}
			return nil
		},
	)
	if err != nil {
		return AuthenticationResult{}, fmt.Errorf("cadastrar cliente: %w", err)
	}

	return newAuthenticationResult(
		created,
		accessToken,
		accessExpiresAt,
		rawRefreshToken,
		refreshToken,
	), nil
}

// Login normaliza e valida as credenciais, compara a senha e abre uma nova
// sessão. Usuário ausente e senha incorreta produzem a mesma resposta para não
// revelar a existência de uma conta.
func (model *AuthenticationModel) Login(
	ctx context.Context,
	input LoginInput,
) (AuthenticationResult, error) {
	email := NormalizeCustomerEmail(input.Email)
	if err := ValidateCustomerEmail(email); err != nil {
		return AuthenticationResult{}, err
	}
	if err := ValidateCustomerPassword(input.Password); err != nil {
		return AuthenticationResult{}, err
	}

	customer, err := model.records.FindByEmail(ctx, email)
	if errors.Is(err, ErrCustomerNotFound) {
		model.passwordHasher.Matches(input.Password, model.passwordHasher.DummyHash())
		return AuthenticationResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return AuthenticationResult{}, fmt.Errorf("buscar credenciais: %w", err)
	}
	if !model.passwordHasher.Matches(input.Password, customer.PasswordHash) {
		return AuthenticationResult{}, ErrInvalidCredentials
	}

	return model.issueTokenPair(ctx, customer)
}

// Refresh valida o token apresentado, gera seu substituto e solicita a rotação
// atômica da sessão. O token de acesso é emitido com o cliente relido enquanto a
// linha está bloqueada, evitando usar dados alterados ao mesmo tempo.
func (model *AuthenticationModel) Refresh(
	ctx context.Context,
	rawToken string,
) (AuthenticationResult, error) {
	currentHash, err := model.refreshTokenIssuer.Hash(rawToken)
	if err != nil {
		return AuthenticationResult{}, err
	}

	rawReplacement, replacement, err := model.refreshTokenIssuer.Generate()
	if err != nil {
		return AuthenticationResult{}, fmt.Errorf("emitir refresh token: %w", err)
	}
	// A assinatura acontece antes de confirmar a rotação; assim uma falha não
	// consome o refresh token atual sem entregar um novo par de credenciais.
	var accessToken string
	var accessExpiresAt time.Time
	customer, replacement, err := model.records.RotateRefreshToken(
		ctx,
		currentHash,
		replacement,
		func(lockedCustomer Customer) error {
			var issueErr error
			accessToken, accessExpiresAt, issueErr = model.accessTokenIssuer.Generate(lockedCustomer)
			if issueErr != nil {
				return fmt.Errorf("emitir token de acesso: %w", issueErr)
			}
			return nil
		},
	)
	if err != nil {
		return AuthenticationResult{}, fmt.Errorf("rotacionar refresh token: %w", err)
	}

	return newAuthenticationResult(
		customer,
		accessToken,
		accessExpiresAt,
		rawReplacement,
		replacement,
	), nil
}

// Logout encerra a sessão associada ao refresh token. A operação é
// intencionalmente idempotente: tokens malformados, desconhecidos ou já
// revogados deixam o chamador deslogado e, por isso, resultam em sucesso.
func (model *AuthenticationModel) Logout(ctx context.Context, rawToken string) error {
	tokenHash, err := model.refreshTokenIssuer.Hash(rawToken)
	if err != nil {
		return nil
	}
	if err := model.records.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return fmt.Errorf("revogar sessão: %w", err)
	}
	return nil
}

// issueTokenPair cria uma família de refresh tokens para um cliente cuja senha
// já foi conferida. A releitura sob bloqueio impede abrir sessão com credenciais
// que mudaram entre a consulta e a transação; retorna os tokens e seus metadados.
func (model *AuthenticationModel) issueTokenPair(
	ctx context.Context,
	authenticatedCustomer Customer,
) (AuthenticationResult, error) {
	rawRefreshToken, refreshToken, err := model.newRefreshToken(authenticatedCustomer.ID)
	if err != nil {
		return AuthenticationResult{}, err
	}

	// O banco relê e bloqueia o cliente antes de executar esta função. Comparar o
	// estado evita abrir sessão se senha ou identidade mudaram após a consulta inicial.
	var accessToken string
	var accessExpiresAt time.Time
	customer, refreshToken, err := model.records.CreateRefreshToken(
		ctx,
		refreshToken,
		func(lockedCustomer Customer) error {
			if lockedCustomer.ID != authenticatedCustomer.ID ||
				lockedCustomer.Email != authenticatedCustomer.Email ||
				lockedCustomer.PasswordHash != authenticatedCustomer.PasswordHash {
				return ErrInvalidCredentials
			}
			var issueErr error
			accessToken, accessExpiresAt, issueErr = model.accessTokenIssuer.Generate(lockedCustomer)
			if issueErr != nil {
				return fmt.Errorf("emitir token de acesso: %w", issueErr)
			}
			return nil
		},
	)
	if err != nil {
		if errors.Is(err, ErrRefreshTokenInvalid) {
			err = ErrInvalidCredentials
		}
		return AuthenticationResult{}, fmt.Errorf("salvar refresh token: %w", err)
	}

	return newAuthenticationResult(
		customer,
		accessToken,
		accessExpiresAt,
		rawRefreshToken,
		refreshToken,
	), nil
}

// newRefreshToken gera a credencial de renovação e associa seus metadados ao
// cliente e a uma nova família de sessão identificada por UUID v7.
func (model *AuthenticationModel) newRefreshToken(
	customerID uuid.UUID,
) (string, RefreshToken, error) {
	rawToken, token, err := model.refreshTokenIssuer.Generate()
	if err != nil {
		return "", RefreshToken{}, fmt.Errorf("emitir refresh token: %w", err)
	}

	familyID, err := uuid.NewV7()
	if err != nil {
		return "", RefreshToken{}, fmt.Errorf("gerar id da sessão: %w", err)
	}
	token.CustomerID = customerID
	token.FamilyID = familyID
	return rawToken, token, nil
}

// newAuthenticationResult monta o resultado do Model com as datas finais
// confirmadas pelo banco, sem recalcular ou alterar os tokens recebidos.
func newAuthenticationResult(
	customer Customer,
	accessToken string,
	accessExpiresAt time.Time,
	rawRefreshToken string,
	refreshToken RefreshToken,
) AuthenticationResult {
	return AuthenticationResult{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     rawRefreshToken,
		RefreshExpiresAt: refreshToken.ExpiresAt,
		SessionExpiresAt: refreshToken.FamilyExpiresAt,
		Customer:         customer,
	}
}
