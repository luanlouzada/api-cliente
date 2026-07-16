package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

const (
	refreshTokenPrefix       = "rt_"
	refreshTokenEntropyBytes = 32
)

var (
	ErrRefreshTokenIdleTTLInvalid     = errors.New("tempo de inatividade do refresh token deve ser positivo")
	ErrRefreshTokenAbsoluteTTLInvalid = errors.New("tempo absoluto da sessão deve ser positivo")
	ErrRefreshTokenLifetimeInvalid    = errors.New("tempo absoluto da sessão deve ser maior ou igual ao tempo de inatividade")
)

// RefreshTokenManager gera credenciais aleatórias de renovação e seus hashes,
// aplicando os limites de inatividade e duração absoluta da sessão.
type RefreshTokenManager struct {
	idleTTL     time.Duration
	absoluteTTL time.Duration
	now         func() time.Time
	reader      io.Reader
}

// NewRefreshTokenManager valida os tempos de inatividade e duração absoluta e
// devolve um gerenciador que usa a fonte criptográfica do sistema operacional.
func NewRefreshTokenManager(idleTTL, absoluteTTL time.Duration) (*RefreshTokenManager, error) {
	if idleTTL <= 0 {
		return nil, ErrRefreshTokenIdleTTLInvalid
	}
	if absoluteTTL <= 0 {
		return nil, ErrRefreshTokenAbsoluteTTLInvalid
	}
	if absoluteTTL < idleTTL {
		return nil, ErrRefreshTokenLifetimeInvalid
	}
	return &RefreshTokenManager{
		idleTTL:     idleTTL,
		absoluteTTL: absoluteTTL,
		now:         time.Now,
		reader:      rand.Reader,
	}, nil
}

// Generate cria um refresh token com 256 bits de entropia e os metadados que
// podem ser persistidos. Retorna o valor público uma única vez e mantém apenas
// seu hash no Model; nenhuma expiração ultrapassa a duração da família.
func (manager *RefreshTokenManager) Generate() (string, RefreshToken, error) {
	// crypto/rand obtém aleatoriedade do sistema operacional. io.ReadFull exige
	// os 32 bytes completos; uma leitura parcial não produziria uma credencial.
	randomBytes := make([]byte, refreshTokenEntropyBytes)
	if _, err := io.ReadFull(manager.reader, randomBytes); err != nil {
		return "", RefreshToken{}, fmt.Errorf("gerar refresh token: %w", err)
	}

	// RawURLEncoding gera texto seguro para JSON e URLs, sem o preenchimento "=".
	// O valor puro volta ao cliente uma vez; somente seu hash seguirá ao banco.
	rawToken := refreshTokenPrefix + base64.RawURLEncoding.EncodeToString(randomBytes)
	now := manager.now().UTC()
	familyExpiresAt := now.Add(manager.absoluteTTL)
	expiresAt := now.Add(manager.idleTTL)
	if expiresAt.After(familyExpiresAt) {
		expiresAt = familyExpiresAt
	}

	tokenID, err := uuid.NewV7()
	if err != nil {
		return "", RefreshToken{}, fmt.Errorf("gerar id do refresh token: %w", err)
	}

	return rawToken, RefreshToken{
		ID:              tokenID,
		TokenHash:       hashRefreshToken(rawToken),
		ExpiresAt:       expiresAt,
		FamilyExpiresAt: familyExpiresAt,
		CreatedAt:       now,
	}, nil
}

// Hash valida prefixo, codificação e quantidade de bytes aleatórios do refresh
// token público antes de produzir o hash SHA-256 usado na busca no banco.
func (manager *RefreshTokenManager) Hash(rawToken string) ([]byte, error) {
	encodedEntropyLength := base64.RawURLEncoding.EncodedLen(refreshTokenEntropyBytes)
	if len(rawToken) != len(refreshTokenPrefix)+encodedEntropyLength ||
		rawToken[:len(refreshTokenPrefix)] != refreshTokenPrefix {
		return nil, ErrRefreshTokenInvalid
	}

	decoded, err := base64.RawURLEncoding.DecodeString(rawToken[len(refreshTokenPrefix):])
	if err != nil || len(decoded) != refreshTokenEntropyBytes {
		return nil, ErrRefreshTokenInvalid
	}
	return hashRefreshToken(rawToken), nil
}

// hashRefreshToken calcula uma cópia independente do hash SHA-256 do token.
// A função pressupõe que o formato já foi validado pelo chamador quando necessário.
func hashRefreshToken(rawToken string) []byte {
	hashArray := sha256.Sum256([]byte(rawToken))
	hash := make([]byte, len(hashArray))
	copy(hash, hashArray[:])
	return hash
}
