package controller

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"cliente-api/internal/view"
)

// requestTimeout isola a resposta dos handlers seguintes até que o processamento termine.
// Um handler atrasado não consegue publicar sucesso depois do prazo, e respostas de
// timeout mantêm o mesmo contrato JSON usado pelos demais erros.
func requestTimeout(duration time.Duration, logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			// O novo contexto comunica o prazo ao Model e ao banco por toda a
			// cadeia da requisição; defer cancel libera o timer assim que ela termina.
			ctx, cancel := context.WithTimeout(request.Context(), duration)
			defer cancel()

			// O handler roda em outra goroutine para que este middleware possa
			// observar o prazo. Sua resposta fica isolada no buffer: se o prazo vencer,
			// uma escrita tardia não consegue misturar sucesso com o erro 504 já enviado.
			buffer := newBufferedResponseWriter()
			done := make(chan struct{})
			go func() {
				defer close(done)
				next.ServeHTTP(buffer, request.WithContext(ctx))
			}()

			select {
			case <-done:
				if ctx.Err() != nil {
					writeRequestContextError(logger, w, ctx.Err())
					return
				}
				if err := buffer.commit(w, request.Method == http.MethodHead); err != nil {
					logger.Error("falha ao escrever a resposta armazenada", "error", err)
				}
			case <-ctx.Done():
				writeRequestContextError(logger, w, ctx.Err())
			}
		})
	}
}

// writeRequestContextError converte cancelamento ou prazo excedido no envelope JSON da API.
// Respostas dessa natureza não são armazenadas em cache porque podem ocorrer em rotas sensíveis.
func writeRequestContextError(logger *slog.Logger, w http.ResponseWriter, err error) {
	status, code, message, _ := classifyError(err)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if writeErr := view.WriteError(w, status, code, message); writeErr != nil {
		logger.Error("falha ao escrever o erro de contexto da requisição", "error", writeErr)
	}
}

// bufferedResponseWriter retém uma resposta até confirmar que a requisição terminou no prazo.
type bufferedResponseWriter struct {
	header      http.Header
	body        bytes.Buffer
	statusCode  int
	wroteHeader bool
}

// newBufferedResponseWriter cria um ResponseWriter isolado com status padrão 200.
func newBufferedResponseWriter() *bufferedResponseWriter {
	return &bufferedResponseWriter{
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

// Header devolve os cabeçalhos temporários exigidos pelo contrato http.ResponseWriter.
func (writer *bufferedResponseWriter) Header() http.Header {
	return writer.header
}

// WriteHeader memoriza somente o primeiro status, reproduzindo a semântica de http.ResponseWriter.
func (writer *bufferedResponseWriter) WriteHeader(statusCode int) {
	if writer.wroteHeader {
		return
	}
	writer.statusCode = statusCode
	writer.wroteHeader = true
}

// Write armazena o corpo no buffer e assume status 200 quando nenhum cabeçalho foi escrito.
func (writer *bufferedResponseWriter) Write(payload []byte) (int, error) {
	if !writer.wroteHeader {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.body.Write(payload)
}

// reset descarta status, cabeçalhos e corpo ainda não publicados para que a
// recuperação de um pânico possa produzir uma resposta de erro limpa.
func (writer *bufferedResponseWriter) reset() {
	clear(writer.header)
	writer.body.Reset()
	writer.statusCode = http.StatusOK
	writer.wroteHeader = false
}

// commit publica atomicamente cabeçalhos, status e corpo no ResponseWriter real.
// Em requisições HEAD, preserva o Content-Length da representação sem enviar o corpo.
func (writer *bufferedResponseWriter) commit(
	destination http.ResponseWriter,
	omitBody bool,
) error {
	for key, values := range writer.header {
		destination.Header()[key] = append([]string(nil), values...)
	}
	if omitBody &&
		responseStatusAllowsBody(writer.statusCode) &&
		destination.Header().Get("Content-Length") == "" {
		destination.Header().Set("Content-Length", strconv.Itoa(writer.body.Len()))
	}
	destination.WriteHeader(writer.statusCode)
	if omitBody || !responseStatusAllowsBody(writer.statusCode) {
		return nil
	}
	_, err := destination.Write(writer.body.Bytes())
	return err
}

// responseStatusAllowsBody informa se o status HTTP pode transportar corpo de resposta.
func responseStatusAllowsBody(statusCode int) bool {
	return statusCode >= 200 &&
		statusCode != http.StatusNoContent &&
		statusCode != http.StatusNotModified
}
