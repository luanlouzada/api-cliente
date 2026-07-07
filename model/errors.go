package model

import "errors"

// ErrCustomerNotFound representa a regra de dominio para cliente inexistente.
var ErrCustomerNotFound = errors.New("cliente nao encontrado")

// ErrCustomerNameRequired representa a regra de dominio para nome obrigatorio.
var ErrCustomerNameRequired = errors.New("nome e obrigatorio")

var ErrCustomerNameTooShort = errors.New("nome deve ter pelo menos 3 caracteres")

var ErrCustomerNameTooLong = errors.New("nome deve ter no maximo 255 caracteres")

// ErrCustomerEmailRequired representa a regra de dominio para email obrigatorio.
var ErrCustomerEmailRequired = errors.New("email e obrigatorio")

var ErrCustomerEmailInvalid = errors.New("email deve ser valido")

var ErrCustomerEmailTooLong = errors.New("email deve ter no maximo 255 caracteres")

// ErrCustomerPhoneRequired representa a regra de dominio para telefone obrigatorio.
var ErrCustomerPhoneRequired = errors.New("telefone e obrigatorio")

var ErrCustomerPhoneInvalid = errors.New("telefone deve ser valido")

var ErrCustomerPhoneTooLong = errors.New("telefone deve ter no maximo 30 caracteres")

var ErrCustomerEmailAlreadyExists = errors.New("email ja existe")
