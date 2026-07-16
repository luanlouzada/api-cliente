package model

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// customerRecords lista somente as operações de persistência necessárias às
// regras de clientes. O contrato reduz o acoplamento com PostgreSQL e permite
// exercitar o Model com implementações controladas.
type customerRecords interface {
	// Create persiste um novo cliente e devolve o estado confirmado pelo banco.
	Create(context.Context, Customer) (Customer, error)
	// FindAll devolve todos os clientes ou um erro da implementação de persistência.
	FindAll(context.Context) ([]Customer, error)
	// FindByID busca um cliente pelo UUID e diferencia ausência por erro de domínio.
	FindByID(context.Context, uuid.UUID) (Customer, error)
	// Update substitui os dados editáveis do cliente e devolve o estado persistido.
	Update(context.Context, uuid.UUID, Customer) (Customer, error)
	// Delete remove o cliente identificado pelo UUID.
	Delete(context.Context, uuid.UUID) error
}

// authenticationRecords reúne as operações de persistência usadas nos fluxos
// de cadastro, login, renovação e encerramento de sessão. O contrato deixa as
// regras independentes dos detalhes de execução de cada consulta PostgreSQL.
type authenticationRecords interface {
	// CreateWithRefreshToken grava cliente e sessão inicial na mesma transação.
	// A função recebida valida o cliente persistido; um erro impede a confirmação.
	CreateWithRefreshToken(
		context.Context,
		Customer,
		RefreshToken,
		func(Customer) error,
	) (Customer, RefreshToken, error)
	// FindByEmail recupera as credenciais persistidas pelo e-mail normalizado.
	FindByEmail(context.Context, string) (Customer, error)
	// CreateRefreshToken abre uma sessão para um cliente existente e executa a
	// função recebida dentro da transação, com o cliente relido sob bloqueio.
	CreateRefreshToken(
		context.Context,
		RefreshToken,
		func(Customer) error,
	) (Customer, RefreshToken, error)
	// RotateRefreshToken troca a credencial atual por uma nova de forma atômica.
	// A função recebida roda depois dos bloqueios necessários; retornar erro cancela
	// a persistência, mantendo a emissão do token de acesso e a sessão atômicas.
	RotateRefreshToken(
		context.Context,
		[]byte,
		RefreshToken,
		func(Customer) error,
	) (Customer, RefreshToken, error)
	// RevokeRefreshToken encerra a família de sessão associada ao hash informado.
	RevokeRefreshToken(context.Context, []byte) error
}

// passwordProtector descreve as operações de senha usadas pelas regras do Model.
type passwordProtector interface {
	// Hash transforma a senha em texto puro em uma representação segura.
	Hash(string) (string, error)
	// Matches informa se a senha corresponde ao hash armazenado.
	Matches(password, passwordHash string) bool
	// DummyHash fornece um hash válido usado para igualar o custo de logins inválidos.
	DummyHash() string
}

// accessTokenGenerator descreve a criação do JWT usado para autorizar requisições.
type accessTokenGenerator interface {
	// Generate emite o token para o estado confirmado do cliente e devolve sua expiração.
	Generate(Customer) (string, time.Time, error)
}

// refreshTokenGenerator descreve a geração e o hash das credenciais de renovação.
type refreshTokenGenerator interface {
	// Generate cria o valor público e os metadados seguros de um refresh token.
	Generate() (string, RefreshToken, error)
	// Hash valida o formato do valor público e produz o hash usado na busca.
	Hash(string) ([]byte, error)
}
