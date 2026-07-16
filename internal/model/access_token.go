package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	jwtAlgorithm      = "HS256"
	minSecretLength   = 32
	minAccessTokenTTL = 10 * time.Second
)

var (
	ErrJWTSecretTooShort     = errors.New("JWT_SECRET deve ter pelo menos 32 bytes")
	ErrAccessTokenTTLInvalid = errors.New(
		"tempo de expiração do JWT deve ser de pelo menos 10 segundos",
	)
	ErrAccessTokenInvalid = errors.New("token de acesso inválido")
	ErrAccessTokenExpired = errors.New("token de acesso expirado")
)

// Claims contém os campos validados extraídos de um token de acesso. No padrão
// JSON Web Token (JWT), esses campos assinados recebem o nome de claims.
type Claims struct {
	Subject   string
	Email     string
	Name      string
	Role      CustomerRole
	IssuedAt  int64
	ExpiresAt int64
}

// accessTokenClaims define o formato interno assinado no JWT.
type accessTokenClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

// AccessTokenManager emite e valida JSON Web Tokens (JWTs). O algoritmo HS256
// usa a mesma chave secreta para criar e conferir a assinatura de cada token.
type AccessTokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewAccessTokenManager valida a chave e o tempo de vida recebidos e devolve um
// gerenciador pronto para emitir JWTs. Configurações fracas são rejeitadas cedo.
func NewAccessTokenManager(secret string, ttl time.Duration) (*AccessTokenManager, error) {
	if len(secret) < minSecretLength {
		return nil, ErrJWTSecretTooShort
	}
	if ttl < minAccessTokenTTL {
		return nil, ErrAccessTokenTTLInvalid
	}
	return &AccessTokenManager{secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}

// Generate cria e assina um JWT para o cliente informado. O método exige ID,
// e-mail não vazio e papel conhecido, e retorna também a data de expiração exata
// comunicada ao cliente.
func (manager *AccessTokenManager) Generate(customer Customer) (string, time.Time, error) {
	if customer.ID == uuid.Nil || strings.TrimSpace(customer.Email) == "" {
		return "", time.Time{}, ErrAccessTokenInvalid
	}
	// NumericDate, o formato temporal do JWT, trabalha em segundos. Normalizar
	// para UTC e remover frações mantém o valor retornado igual ao valor assinado.
	now := manager.now().UTC().Truncate(time.Second)
	expiresAt := now.Add(manager.ttl).Truncate(time.Second)
	role := customer.Role
	if role != CustomerRoleCustomer && role != CustomerRoleAdmin {
		return "", time.Time{}, ErrAccessTokenInvalid
	}
	claims := accessTokenClaims{
		Email: customer.Email,
		Name:  customer.Name,
		Role:  string(role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   customer.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	// O JWT é assinado, não criptografado: qualquer pessoa pode ler seus campos,
	// mas somente quem possui a chave consegue produzir uma assinatura válida.
	signedToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(manager.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("assinar JWT: %w", err)
	}
	return signedToken, expiresAt, nil
}

// Validate verifica assinatura HS256, algoritmo, expiração e campos obrigatórios
// do JWT recebido. Retorna claims confiáveis ou erros públicos distintos para
// token expirado e token inválido.
func (manager *AccessTokenManager) Validate(tokenString string) (Claims, error) {
	parsedClaims := accessTokenClaims{}
	// ParseWithClaims verifica formato e assinatura. As opções restringem o
	// algoritmo a HS256, exigem expiração, validam a data de emissão e usam a
	// fonte de tempo do gerenciador, mantendo emissão e validação coerentes.
	token, err := jwt.ParseWithClaims(
		tokenString,
		&parsedClaims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwtAlgorithm {
				return nil, fmt.Errorf("algoritmo inesperado: %s", token.Method.Alg())
			}
			return manager.secret, nil
		},
		jwt.WithValidMethods([]string{jwtAlgorithm}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(func() time.Time { return manager.now().UTC() }),
	)

	if errors.Is(err, jwt.ErrTokenExpired) {
		return Claims{}, ErrAccessTokenExpired
	}
	if err != nil || token == nil || !token.Valid {
		return Claims{}, ErrAccessTokenInvalid
	}
	subjectID, subjectErr := uuid.Parse(parsedClaims.Subject)
	if subjectErr != nil || subjectID == uuid.Nil ||
		parsedClaims.Email == "" || parsedClaims.IssuedAt == nil || parsedClaims.ExpiresAt == nil {
		return Claims{}, ErrAccessTokenInvalid
	}
	role := CustomerRole(parsedClaims.Role)
	if role != CustomerRoleCustomer && role != CustomerRoleAdmin {
		return Claims{}, ErrAccessTokenInvalid
	}

	return Claims{
		Subject:   parsedClaims.Subject,
		Email:     parsedClaims.Email,
		Name:      parsedClaims.Name,
		Role:      role,
		IssuedAt:  parsedClaims.IssuedAt.Unix(),
		ExpiresAt: parsedClaims.ExpiresAt.Unix(),
	}, nil
}
