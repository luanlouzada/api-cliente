package model

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken contém somente os metadados seguros de uma credencial de
// renovação. O Model nunca persiste o token em texto puro; armazena somente seu
// hash. FamilyID agrupa todas as rotações de uma mesma sessão.
type RefreshToken struct {
	ID              uuid.UUID
	CustomerID      uuid.UUID
	FamilyID        uuid.UUID
	TokenHash       []byte
	ExpiresAt       time.Time
	FamilyExpiresAt time.Time
	CreatedAt       time.Time
}
