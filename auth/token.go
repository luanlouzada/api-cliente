package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cliente-api/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	jwtAlgorithm = "HS256"

	minSecretLength = 32
)

var (
	ErrJWTSecretTooShort = errors.New("JWT_SECRET deve ter pelo menos 32 caracteres")

	ErrJWTAccessTokenTTLInvalid = errors.New("tempo de expiracao do JWT deve ser positivo")

	ErrBearerTokenRequired = errors.New("authorization deve usar Bearer token")

	ErrAccessTokenInvalid = errors.New("token de acesso invalido")

	ErrAccessTokenExpired = errors.New("token de acesso expirado")
)

type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

type Claims struct {
	Subject   string `json:"sub"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type tokenClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`

	jwt.RegisteredClaims
}

type contextKey string

const claimsContextKey contextKey = "auth_claims"

func NewTokenManager(secret string, ttl time.Duration) (*TokenManager, error) {

	if len(secret) < minSecretLength {

		return nil, ErrJWTSecretTooShort
	}

	if ttl <= 0 {

		return nil, ErrJWTAccessTokenTTLInvalid
	}

	return &TokenManager{
		secret: []byte(secret),
		ttl:    ttl,
		now:    time.Now,
	}, nil
}

func (manager *TokenManager) Generate(customer model.Customer) (string, time.Time, error) {

	now := manager.now().UTC()

	expiresAt := now.Add(manager.ttl)

	claims := tokenClaims{
		Email: customer.Email,
		Name:  customer.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   customer.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(manager.secret)

	if err != nil {

		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}

	return signedToken, expiresAt, nil
}

func (manager *TokenManager) Validate(tokenString string) (Claims, error) {

	parsedClaims := tokenClaims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&parsedClaims,
		func(token *jwt.Token) (interface{}, error) {

			if token.Method.Alg() != jwtAlgorithm {

				return nil, fmt.Errorf("algoritmo inesperado: %s", token.Method.Alg())
			}

			return manager.secret, nil
		},
		jwt.WithValidMethods([]string{jwtAlgorithm}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(func() time.Time {

			return manager.now().UTC()
		}),
	)

	if errors.Is(err, jwt.ErrTokenExpired) {
		return Claims{}, ErrAccessTokenExpired
	}

	if err != nil || token == nil || !token.Valid {
		return Claims{}, ErrAccessTokenInvalid
	}

	if _, err := uuid.Parse(parsedClaims.Subject); err != nil {
		return Claims{}, ErrAccessTokenInvalid
	}

	if parsedClaims.Email == "" || parsedClaims.IssuedAt == nil || parsedClaims.ExpiresAt == nil {
		return Claims{}, ErrAccessTokenInvalid
	}

	return Claims{
		Subject:   parsedClaims.Subject,
		Email:     parsedClaims.Email,
		Name:      parsedClaims.Name,
		IssuedAt:  parsedClaims.IssuedAt.Unix(),
		ExpiresAt: parsedClaims.ExpiresAt.Unix(),
	}, nil
}

func (manager *TokenManager) Middleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		token, ok := bearerToken(r.Header.Get("Authorization"))

		if !ok {
			http.Error(w, ErrBearerTokenRequired.Error(), http.StatusUnauthorized)
			return
		}

		claims, err := manager.Validate(token)

		if errors.Is(err, ErrAccessTokenExpired) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		if err != nil {
			http.Error(w, ErrAccessTokenInvalid.Error(), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey, claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ClaimsFromContext(ctx context.Context) (Claims, bool) {

	claims, ok := ctx.Value(claimsContextKey).(Claims)

	return claims, ok
}

func bearerToken(authorization string) (string, bool) {

	scheme, token, ok := strings.Cut(strings.TrimSpace(authorization), " ")

	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}

	return strings.TrimSpace(token), true
}
