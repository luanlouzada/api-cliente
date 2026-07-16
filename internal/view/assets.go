package view

import (
	"embed"
	"io/fs"
)

// A diretiva go:embed pede ao compilador que copie os arquivos do frontend para
// dentro do executável; a aplicação pode servi-los sem depender de uma pasta externa.
//
//go:embed frontend/*
var assets embed.FS

// Files devolve apenas a subárvore pública do frontend incorporado ao binário.
// Restringir a raiz impede que caminhos internos do pacote sejam servidos por engano.
func Files() (fs.FS, error) {
	return fs.Sub(assets, "frontend")
}
