package model

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	customerNameMinLength  = 3
	customerNameMaxLength  = 255
	customerEmailMaxLength = 255
	customerPhoneMaxLength = 30
	customerPhoneMinDigits = 10
	customerPhoneMaxDigits = 15
)

var (
	customerEmailRegex       = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)
	customerPhoneFormatRegex = regexp.MustCompile(`^\+?[0-9\s().-]+$`)
)

// Customer representa a entidade principal da API.
type Customer struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewCustomer cria um cliente valido a partir dos dados de entrada.
func NewCustomer(name, email, phone string) (Customer, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)

	if err := validateCustomerName(name); err != nil {
		return Customer{}, err
	}
	if err := validateCustomerEmail(email); err != nil {
		return Customer{}, err
	}
	if err := validateCustomerPhone(phone); err != nil {
		return Customer{}, err
	}

	return Customer{
		Name:  name,
		Email: email,
		Phone: phone,
	}, nil
}

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

func validateCustomerEmail(email string) error {
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
	for _, char := range phone {
		if char >= '0' && char <= '9' {
			digitCount++
		}
	}
	if digitCount < customerPhoneMinDigits || digitCount > customerPhoneMaxDigits {
		return ErrCustomerPhoneInvalid
	}

	return nil
}

// CreateCustomerRequest representa o corpo esperado ao criar um cliente.
type CreateCustomerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// UpdateCustomerRequest representa o corpo esperado ao atualizar um cliente.
type UpdateCustomerRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}
