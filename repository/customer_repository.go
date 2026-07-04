package repository

import (
	"cliente-api/model"
	"sort"
	"sync"
	"time"
)

// CustomerRepository guarda os clientes em memoria.
type CustomerRepository struct {
	// RWMutex evita corrida de dados quando houver varias requisicoes ao mesmo tempo.
	mu        sync.RWMutex
	customers map[int]model.Customer
	nextID    int
}

// NewCustomerRepository cria um repositorio vazio.
func NewCustomerRepository() *CustomerRepository {
	return &CustomerRepository{
		customers: make(map[int]model.Customer),
		nextID:    1,
	}
}

// Create adiciona um cliente e simula ID auto incremental e timestamps de banco.
func (repo *CustomerRepository) Create(customer model.Customer) model.Customer {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	now := time.Now().UTC()

	customer.ID = repo.nextID
	customer.CreatedAt = now
	customer.UpdatedAt = now
	repo.customers[customer.ID] = customer
	repo.nextID++

	return customer
}

// FindAll retorna todos os clientes cadastrados.
func (repo *CustomerRepository) FindAll() []model.Customer {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	// Copia os valores do map para uma slice, formato mais adequado para JSON.
	customers := make([]model.Customer, 0, len(repo.customers))
	for _, customer := range repo.customers {
		customers = append(customers, customer)
	}

	// Maps em Go nao garantem ordem; ordenar facilita exemplos e testes.
	sort.Slice(customers, func(i, j int) bool {
		return customers[i].ID < customers[j].ID
	})

	return customers
}

// FindByID busca um cliente pelo ID.
func (repo *CustomerRepository) FindByID(id int) (model.Customer, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	customer, ok := repo.customers[id]
	if !ok {
		return model.Customer{}, model.ErrCustomerNotFound
	}

	return customer, nil
}

// Update substitui os dados de um cliente existente e renova o updated_at.
func (repo *CustomerRepository) Update(id int, customer model.Customer) (model.Customer, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	currentCustomer, ok := repo.customers[id]
	if !ok {
		return model.Customer{}, model.ErrCustomerNotFound
	}

	// O ID vem da URL e deve continuar sendo o identificador oficial.
	customer.ID = id
	// CreatedAt nao muda em uma atualizacao; UpdatedAt registra a ultima alteracao.
	customer.CreatedAt = currentCustomer.CreatedAt
	customer.UpdatedAt = time.Now().UTC()
	repo.customers[id] = customer

	return customer, nil
}

// Delete remove um cliente existente.
func (repo *CustomerRepository) Delete(id int) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if _, ok := repo.customers[id]; !ok {
		return model.ErrCustomerNotFound
	}

	delete(repo.customers, id)
	return nil
}
