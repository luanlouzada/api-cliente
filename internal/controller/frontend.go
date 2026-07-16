package controller

import (
	"io/fs"
	"log/slog"
	"net/http"
)

// FrontendController entrega a View web incorporada ao mesmo binário da API.
type FrontendController struct {
	files      fs.FS
	fileServer http.Handler
	logger     *slog.Logger
}

// NewFrontendController prepara a entrega do HTML e dos arquivos estáticos.
func NewFrontendController(files fs.FS, logger *slog.Logger) *FrontendController {
	if logger == nil {
		logger = slog.Default()
	}
	return &FrontendController{
		files:      files,
		fileServer: http.FileServer(http.FS(files)),
		logger:     logger,
	}
}

// Index serve a página principal incorporada no executável e registra erros de leitura ou escrita.
func (controller *FrontendController) Index(w http.ResponseWriter, request *http.Request) {
	page, err := fs.ReadFile(controller.files, "index.html")
	if err != nil {
		writeApplicationError(controller.logger, w, request, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(page); err != nil {
		controller.logger.Error("falha ao escrever o frontend", "error", err)
	}
}

// Assets devolve o handler de arquivos estáticos já limitado ao sistema de arquivos do frontend.
func (controller *FrontendController) Assets() http.Handler {
	return controller.fileServer
}
