package main

import (
	"fmt"
	"os"

	"cliente-api/internal/config"
)

// main apresenta a URL navegável da interface web sem exigir os demais segredos
// necessários para iniciar a API completa.
func main() {
	frontendURL, err := config.LoadFrontendURL()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Interface web:", frontendURL)
}
