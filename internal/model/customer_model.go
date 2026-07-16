package model

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateCustomerInput reúne os dados que a operação de criação do Model aceita.
type CreateCustomerInput struct {
	Name     string
	Email    string
	Phone    string
	Password string
}

// UpdateCustomerInput reúne somente os campos editáveis de um cliente.
type UpdateCustomerInput struct {
	Name  string
	Email string
	Phone string
}

// Principal representa a identidade autenticada usada nas decisões de autorização.
type Principal struct {
	CustomerID uuid.UUID
	Role       CustomerRole
}

// CustomerModel reúne as regras, a autorização e a persistência de clientes.
type CustomerModel struct {
	records        customerRecords
	passwordHasher passwordProtector
}

// NewCustomerModel cria a parte do Model responsável pelos clientes usando o
// PostgreSQL e o mecanismo de senha configurados pela aplicação.
func NewCustomerModel(pool *pgxpool.Pool, passwordHasher BcryptPasswordHasher) *CustomerModel {
	return newCustomerModel(newPostgresData(pool), passwordHasher)
}

// newCustomerModel monta a estrutura a partir dos componentes internos do Model.
func newCustomerModel(records customerRecords, passwordHasher passwordProtector) *CustomerModel {
	return &CustomerModel{records: records, passwordHasher: passwordHasher}
}

// Create valida a permissão do principal, monta e protege os dados recebidos e
// persiste um novo cliente. Somente administradores podem executar a operação.
func (model *CustomerModel) Create(
	ctx context.Context,
	principal Principal,
	input CreateCustomerInput,
) (Customer, error) {
	if principal.Role != CustomerRoleAdmin {
		return Customer{}, ErrForbidden
	}
	customer, err := buildCustomer(input, model.passwordHasher)
	if err != nil {
		return Customer{}, err
	}

	created, err := model.records.Create(ctx, customer)
	if err != nil {
		return Customer{}, fmt.Errorf("criar cliente: %w", err)
	}
	return created, nil
}

// List devolve todos os clientes para um principal administrador. A slice é
// mantida como veio da persistência porque a representação JSON pertence à
// fronteira de apresentação, não às regras do Model.
func (model *CustomerModel) List(ctx context.Context, principal Principal) ([]Customer, error) {
	if principal.Role != CustomerRoleAdmin {
		return nil, ErrForbidden
	}
	customers, err := model.records.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar clientes: %w", err)
	}
	return customers, nil
}

// Get busca um cliente pelo ID quando o principal é administrador ou dono do
// registro. Retorna a entidade encontrada ou um erro contextualizado.
func (model *CustomerModel) Get(
	ctx context.Context,
	principal Principal,
	id uuid.UUID,
) (Customer, error) {
	if !principalCanAccessCustomer(principal, id) {
		return Customer{}, ErrForbidden
	}
	customer, err := model.records.FindByID(ctx, id)
	if err != nil {
		return Customer{}, fmt.Errorf("buscar cliente: %w", err)
	}
	return customer, nil
}

// Update valida autorização e os novos dados do perfil antes de pedir a
// persistência. Administradores podem editar qualquer cliente; clientes, só a si.
func (model *CustomerModel) Update(
	ctx context.Context,
	principal Principal,
	id uuid.UUID,
	input UpdateCustomerInput,
) (Customer, error) {
	if !principalCanAccessCustomer(principal, id) {
		return Customer{}, ErrForbidden
	}
	profile, err := NewCustomerProfile(input.Name, input.Email, input.Phone)
	if err != nil {
		return Customer{}, err
	}

	updated, err := model.records.Update(ctx, id, profile)
	if err != nil {
		return Customer{}, fmt.Errorf("atualizar cliente: %w", err)
	}
	return updated, nil
}

// Delete remove o cliente identificado quando a política dono-ou-administrador
// permite a operação. Erros de banco recebem o contexto da operação.
func (model *CustomerModel) Delete(ctx context.Context, principal Principal, id uuid.UUID) error {
	if !principalCanAccessCustomer(principal, id) {
		return ErrForbidden
	}
	if err := model.records.Delete(ctx, id); err != nil {
		return fmt.Errorf("excluir cliente: %w", err)
	}
	return nil
}

// principalCanAccessCustomer centraliza a política de autorização: administrador
// acessa qualquer registro e cliente comum acessa somente o próprio UUID.
func principalCanAccessCustomer(principal Principal, customerID uuid.UUID) bool {
	return principal.Role == CustomerRoleAdmin ||
		(principal.Role == CustomerRoleCustomer && principal.CustomerID == customerID)
}

// buildCustomer transforma a entrada do Model em um Customer. Primeiro valida os
// dados e só depois calcula o hash, evitando o trabalho custoso do bcrypt quando
// a entrada já é inválida e sem repetir a validação do perfil após o hash.
func buildCustomer(input CreateCustomerInput, passwordHasher passwordProtector) (Customer, error) {
	profile, err := NewCustomerProfile(input.Name, input.Email, input.Phone)
	if err != nil {
		return Customer{}, err
	}
	if err := ValidateCustomerPassword(input.Password); err != nil {
		return Customer{}, err
	}

	passwordHash, err := passwordHasher.Hash(input.Password)
	if err != nil {
		return Customer{}, fmt.Errorf("proteger senha: %w", err)
	}

	return newCustomerFromProfile(profile, passwordHash)
}
