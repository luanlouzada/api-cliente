package model

import "errors"

var ErrCustomerNotFound = errors.New("cliente nao encontrado")

var ErrCustomerNameRequired = errors.New("nome e obrigatorio")

var ErrCustomerNameTooShort = errors.New("nome deve ter pelo menos 3 caracteres")

var ErrCustomerNameTooLong = errors.New("nome deve ter no maximo 255 caracteres")

var ErrCustomerEmailRequired = errors.New("email e obrigatorio")

var ErrCustomerEmailInvalid = errors.New("email deve ser valido")

var ErrCustomerEmailTooLong = errors.New("email deve ter no maximo 255 caracteres")

var ErrCustomerPhoneRequired = errors.New("telefone e obrigatorio")

var ErrCustomerPhoneInvalid = errors.New("telefone deve ser valido")

var ErrCustomerPhoneTooLong = errors.New("telefone deve ter no maximo 30 caracteres")

var ErrCustomerEmailAlreadyExists = errors.New("email ja existe")

var ErrInvalidCustomerID = errors.New("id deve ser um uuid valido")

var ErrCustomerPasswordRequired = errors.New("senha e obrigatoria")

var ErrCustomerPasswordTooShort = errors.New("senha deve ter pelo menos 8 caracteres")

var ErrCustomerPasswordTooLong = errors.New("senha deve ter no maximo 72 bytes")

var ErrCustomerPasswordHashRequired = errors.New("hash da senha e obrigatorio")

var ErrInvalidCredentials = errors.New("credenciais invalidas")
