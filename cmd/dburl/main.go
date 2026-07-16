package main

import (
	"fmt"
	"os"

	"cliente-api/internal/config"
)

// main imprime a URL de banco efetivamente resolvida para que ferramentas de
// migração usem a mesma configuração da API.
func main() {
	databaseURL, err := config.LoadDatabaseURL()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(databaseURL)
}
