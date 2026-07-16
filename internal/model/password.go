package model

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// dummyPasswordHash é um hash bcrypt de um valor que nunca é aceito como
// credencial. Ele faz o login gastar trabalho comparável para usuários existentes
// e ausentes, reduzindo diferenças de tempo que poderiam revelar uma conta.
const dummyPasswordHash = "$2a$10$c7h4ukDiUBzSunvjbUMzMuALvWLyUv3TdTnPvOUUWT1D3hVjssMtC"

// BcryptPasswordHasher implementa a proteção e a comparação de senhas com bcrypt.
type BcryptPasswordHasher struct {
	cost    int
	compare func(hashedPassword, password []byte) error
}

// NewBcryptPasswordHasher devolve um hasher configurado com o custo padrão da
// biblioteca bcrypt, equilibrando segurança e tempo de processamento.
func NewBcryptPasswordHasher() BcryptPasswordHasher {
	return BcryptPasswordHasher{
		cost:    bcrypt.DefaultCost,
		compare: bcrypt.CompareHashAndPassword,
	}
}

// Hash transforma a senha em texto puro em um hash bcrypt com salt aleatório.
// Retorna o valor codificado para persistência ou um erro contextualizado.
func (hasher BcryptPasswordHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), hasher.cost)
	if err != nil {
		return "", fmt.Errorf("gerar hash da senha: %w", err)
	}
	return string(hash), nil
}

// Matches compara uma senha em texto puro com o hash persistido e retorna apenas
// se correspondem, sem expor os detalhes de erro do bcrypt. Quando o hash salvo
// está malformado, executa uma comparação fictícia para evitar uma falha rápida.
func (hasher BcryptPasswordHasher) Matches(password, passwordHash string) bool {
	compare := hasher.compare
	if compare == nil {
		compare = bcrypt.CompareHashAndPassword
	}

	err := compare([]byte(passwordHash), []byte(password))
	if err == nil {
		return true
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false
	}

	// Um erro de formato ou custo ocorre antes do trabalho completo do bcrypt.
	// A comparação adicional aproxima o custo do caminho com um hash válido.
	_ = compare([]byte(dummyPasswordHash), []byte(password))
	return false
}

// DummyHash devolve um hash bcrypt válido para executar uma comparação de custo
// semelhante quando não existe uma senha real armazenada.
func (hasher BcryptPasswordHasher) DummyHash() string {
	return dummyPasswordHash
}
