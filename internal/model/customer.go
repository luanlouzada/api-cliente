package model

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	customerNameMinLength     = 3
	customerNameMaxLength     = 255
	customerEmailMaxLength    = 255
	customerPhoneMaxLength    = 30
	customerPhoneMinDigits    = 10
	customerPhoneMaxDigits    = 15
	customerPasswordMinLength = 8
	customerPasswordMaxBytes  = 72
)

var (
	customerEmailRegex       = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)
	customerPhoneFormatRegex = regexp.MustCompile(`^\+?[0-9(][0-9 ().-]*[0-9)]$`)
)

// CustomerRole representa o nível de permissão atribuído a um cliente.
type CustomerRole string

const (
	CustomerRoleCustomer CustomerRole = "customer"
	CustomerRoleAdmin    CustomerRole = "admin"
)

// Customer representa um cliente dentro do Model. Ele não possui tags JSON
// porque os DTOs e Mappers definem o contrato HTTP. As consultas SQL leem cada
// campo explicitamente, por isso também não são necessárias tags de banco.
type Customer struct {
	ID           uuid.UUID
	Name         string
	Email        string
	Phone        string
	Role         CustomerRole
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewCustomer cria uma entidade pronta para persistência a partir dos dados do
// cliente e de um hash de senha já calculado. Ela normaliza e valida o perfil,
// exige um hash não vazio, gera um UUID v7 e atribui o papel menos privilegiado.
// Retorna a entidade completa ou o primeiro erro de domínio encontrado.
func NewCustomer(name, email, phone, passwordHash string) (Customer, error) {
	customer, err := NewCustomerProfile(name, email, phone)
	if err != nil {
		return Customer{}, err
	}
	if strings.TrimSpace(passwordHash) == "" {
		return Customer{}, ErrCustomerPasswordHashRequired
	}

	id, err := uuid.NewV7()
	if err != nil {
		return Customer{}, err
	}

	customer.ID = id
	customer.Role = CustomerRoleCustomer
	customer.PasswordHash = passwordHash
	return customer, nil
}

// NewCustomerProfile normaliza e valida os campos públicos de um cliente.
// É usada tanto na criação quanto na atualização e, por isso, não atribui ID,
// papel, senha ou datas. Retorna um perfil válido ou um erro de validação.
func NewCustomerProfile(name, email, phone string) (Customer, error) {
	name = strings.TrimSpace(name)
	email = NormalizeCustomerEmail(email)
	phone = strings.TrimSpace(phone)

	if err := validateCustomerName(name); err != nil {
		return Customer{}, err
	}
	if err := ValidateCustomerEmail(email); err != nil {
		return Customer{}, err
	}
	if err := validateCustomerPhone(phone); err != nil {
		return Customer{}, err
	}

	return Customer{Name: name, Email: email, Phone: phone}, nil
}

// NormalizeCustomerEmail remove espaços nas extremidades e converte o e-mail
// para letras minúsculas, produzindo a forma canônica usada nas comparações.
func NormalizeCustomerEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// validateCustomerName verifica se o nome normalizado respeita os limites de
// caracteres Unicode. Retorna nil quando o valor pode integrar a entidade.
func validateCustomerName(name string) error {
	switch length := utf8.RuneCountInString(name); {
	case length == 0:
		return ErrCustomerNameRequired
	case length < customerNameMinLength:
		return ErrCustomerNameTooShort
	case length > customerNameMaxLength:
		return ErrCustomerNameTooLong
	default:
		return nil
	}
}

// ValidateCustomerEmail verifica presença, tamanho e formato do e-mail já
// normalizado. Retorna nil para um valor aceito ou um erro específico de domínio.
func ValidateCustomerEmail(email string) error {
	switch {
	case email == "":
		return ErrCustomerEmailRequired
	case len(email) > customerEmailMaxLength:
		return ErrCustomerEmailTooLong
	case !customerEmailRegex.MatchString(email):
		return ErrCustomerEmailInvalid
	default:
		return nil
	}
}

// validateCustomerPhone aceita os separadores visuais suportados e valida a
// quantidade real de dígitos. Retorna nil quando o telefone é válido.
func validateCustomerPhone(phone string) error {
	switch {
	case phone == "":
		return ErrCustomerPhoneRequired
	case len(phone) > customerPhoneMaxLength:
		return ErrCustomerPhoneTooLong
	case !customerPhoneFormatRegex.MatchString(phone):
		return ErrCustomerPhoneInvalid
	}

	digitCount := 0
	for _, character := range phone {
		if character >= '0' && character <= '9' {
			digitCount++
		}
	}
	if digitCount < customerPhoneMinDigits || digitCount > customerPhoneMaxDigits {
		return ErrCustomerPhoneInvalid
	}

	return nil
}

// ValidateCustomerPassword valida a senha em texto puro antes de gerar o hash.
// O mínimo considera caracteres Unicode, enquanto o máximo usa bytes por causa
// do limite de entrada do bcrypt. Retorna nil quando a senha é aceitável.
func ValidateCustomerPassword(password string) error {
	switch length := utf8.RuneCountInString(password); {
	case strings.TrimSpace(password) == "":
		return ErrCustomerPasswordRequired
	case length < customerPasswordMinLength:
		return ErrCustomerPasswordTooShort
	case len(password) > customerPasswordMaxBytes:
		return ErrCustomerPasswordTooLong
	default:
		return nil
	}
}
