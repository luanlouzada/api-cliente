package model

import "errors"

// ErrCustomerNotFound representa a regra de dominio para cliente inexistente.
var ErrCustomerNotFound = errors.New("cliente nao encontrado")

// ErrCustomerNameRequired representa a regra de dominio para nome obrigatorio.
var ErrCustomerNameRequired = errors.New("nome e obrigatorio")

// ErrCustomerEmailRequired representa a regra de dominio para email obrigatorio.
var ErrCustomerEmailRequired = errors.New("email e obrigatorio")

// ErrCustomerPhoneRequired representa a regra de dominio para telefone obrigatorio.
var ErrCustomerPhoneRequired = errors.New("telefone e obrigatorio")
