package model

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID              uuid.UUID
	CustomerID      uuid.UUID
	FamilyID        uuid.UUID
	TokenHash       []byte
	ExpiresAt       time.Time
	FamilyExpiresAt time.Time
	CreatedAt       time.Time
}
