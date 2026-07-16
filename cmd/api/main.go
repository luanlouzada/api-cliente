package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cliente-api/internal/config"
	"cliente-api/internal/controller"
	"cliente-api/internal/database"
	"cliente-api/internal/model"
	"cliente-api/internal/view"
)

// main carrega a configuração, prepara o encerramento por sinais e inicia a API.
func main() {
	// O pacote slog fornece logs estruturados: cada mensagem pode carregar pares
	// como "error" e seu valor. NewTextHandler define stdout como destino e usa
	// um formato textual legível; slog.New cria o logger usado pelo processo.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	applicationConfig, err := config.Load()
	if err != nil {
		logger.Error("configuração inválida", "error", err)
		os.Exit(1)
	}

	// NotifyContext cancela o contexto raiz ao receber Ctrl+C ou SIGTERM, sinal
	// normalmente enviado por Docker. run usa o cancelamento para encerrar a API.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, applicationConfig, logger); err != nil {
		logger.Error("aplicação encerrada", "error", err)
		os.Exit(1)
	}
}

// run conecta Model, View e Controllers, inicia o servidor e coordena seu
// encerramento gracioso. Toda a montagem do MVC fica visível no ponto de entrada.
func run(ctx context.Context, applicationConfig config.Config, logger *slog.Logger) error {
	connectContext, cancelConnect := context.WithTimeout(ctx, 10*time.Second)
	defer cancelConnect()

	pool, err := database.NewPostgresPool(connectContext, applicationConfig.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	accessTokens, err := model.NewAccessTokenManager(
		applicationConfig.Auth.JWTSecret,
		applicationConfig.Auth.AccessTokenTTL,
	)
	if err != nil {
		return fmt.Errorf("configurar JWT: %w", err)
	}
	refreshTokens, err := model.NewRefreshTokenManager(
		applicationConfig.Auth.RefreshTokenIdleTTL,
		applicationConfig.Auth.RefreshTokenAbsoluteTTL,
	)
	if err != nil {
		return fmt.Errorf("configurar token de renovação: %w", err)
	}
	passwords := model.NewBcryptPasswordHasher()

	// O Model concentra dados e regras. Cada Controller recebe a parte do Model
	// que atende suas rotas e transforma o resultado em uma View HTTP.
	customerModel := model.NewCustomerModel(pool, passwords)
	authenticationModel := model.NewAuthenticationModel(
		pool,
		passwords,
		accessTokens,
		refreshTokens,
	)
	authenticationController := controller.NewAuthenticationController(authenticationModel, logger)
	customerController := controller.NewCustomerController(customerModel, logger)

	frontend, err := view.Files()
	if err != nil {
		return fmt.Errorf("carregar interface web: %w", err)
	}
	frontendController := controller.NewFrontendController(frontend, logger)
	router := controller.NewRouter(controller.RouterDependencies{
		Authentication: authenticationController,
		Customers:      customerController,
		TokenValidator: accessTokens,
		Frontend:       frontendController,
		Logger:         logger,
	})

	server := &http.Server{
		Addr:              net.JoinHostPort(applicationConfig.HTTPHost, applicationConfig.Port),
		Handler:           router,
		ReadHeaderTimeout: applicationConfig.Server.ReadHeaderTimeout,
		ReadTimeout:       applicationConfig.Server.ReadTimeout,
		WriteTimeout:      applicationConfig.Server.WriteTimeout,
		IdleTimeout:       applicationConfig.Server.IdleTimeout,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("abrir endereço HTTP %s: %w", server.Addr, err)
	}

	// Serve bloqueia enquanto a API está ativa. A goroutine, uma função leve que
	// executa concorrentemente, permite aguardar ao mesmo tempo uma falha do
	// servidor e o sinal de encerramento do processo.
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(listener)
	}()

	logger.Info("API iniciada", "address", "http://"+server.Addr)
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("servidor HTTP: %w", err)
	case <-ctx.Done():
		logger.Info("encerrando API")
	}

	// O contexto raiz já foi cancelado. Um contexto novo concede às requisições
	// em andamento uma janela limitada para terminar antes do fechamento forçado.
	shutdownContext, cancelShutdown := context.WithTimeout(
		context.Background(),
		applicationConfig.Server.ShutdownTimeout,
	)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownContext); err != nil {
		_ = server.Close()
		return fmt.Errorf("encerrar servidor HTTP: %w", err)
	}

	if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("servidor HTTP durante encerramento: %w", err)
	}
	return nil
}
