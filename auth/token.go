package auth // Este arquivo faz parte do pacote auth, que cuida de autenticacao.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cliente-api/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	// jwtAlgorithm diz qual algoritmo de assinatura esta API aceita.
	// HS256 usa a mesma chave secreta para assinar e validar o JWT.
	jwtAlgorithm = "HS256"

	// minSecretLength define o tamanho minimo do JWT_SECRET.
	// Uma chave pequena demais fica mais facil de atacar por tentativa e erro.
	minSecretLength = 32
)

var (
	// ErrJWTSecretTooShort aparece quando o segredo do JWT e curto demais.
	ErrJWTSecretTooShort = errors.New("JWT_SECRET deve ter pelo menos 32 caracteres")

	// ErrJWTAccessTokenTTLInvalid aparece quando o tempo de vida do token nao e positivo.
	ErrJWTAccessTokenTTLInvalid = errors.New("tempo de expiracao do JWT deve ser positivo")

	// ErrBearerTokenRequired aparece quando a rota protegida nao recebeu Authorization: Bearer <token>.
	ErrBearerTokenRequired = errors.New("authorization deve usar Bearer token")

	// ErrAccessTokenInvalid aparece quando o token esta quebrado, adulterado ou incompleto.
	ErrAccessTokenInvalid = errors.New("token de acesso invalido")

	// ErrAccessTokenExpired aparece quando o token era valido, mas ja passou da hora de vencer.
	ErrAccessTokenExpired = errors.New("token de acesso expirado")
)

// TokenManager e o gerenciador dos access tokens JWT.
// Ele guarda tudo que precisa para gerar, validar e proteger rotas com JWT.
type TokenManager struct {
	secret []byte           // Chave secreta usada para assinar e validar os tokens.
	ttl    time.Duration    // Tempo de vida do access token, por exemplo 15 minutos.
	now    func() time.Time // Funcao que devolve a hora atual; pode ser substituida nos testes.
}

// Claims e o formato limpo que o restante da API usa depois que o JWT foi validado.
// Pense nisso como os dados confiaveis que foram extraidos do token.
type Claims struct {
	Subject   string `json:"sub"`   // ID do customer autenticado; no JWT esse campo padrao chama "sub".
	Email     string `json:"email"` // Email do customer autenticado.
	Name      string `json:"name"`  // Nome do customer autenticado.
	IssuedAt  int64  `json:"iat"`   // Momento em que o token foi criado, em Unix timestamp.
	ExpiresAt int64  `json:"exp"`   // Momento em que o token expira, em Unix timestamp.
}

// tokenClaims e o molde interno do conteudo do JWT.
// Ele junta campos do nosso sistema com campos oficiais que a biblioteca jwt entende.
type tokenClaims struct {
	Email string `json:"email"` // Campo personalizado: email do customer aparece como "email" no token.
	Name  string `json:"name"`  // Campo personalizado: nome do customer aparece como "name" no token.

	// RegisteredClaims traz campos padrao definidos pela especificacao JWT, como:
	// Subject   -> "sub", normalmente o ID de quem fez login.
	// IssuedAt  -> "iat", momento em que o token foi criado.
	// ExpiresAt -> "exp", momento em que o token vence.
	jwt.RegisteredClaims
}

// contextKey cria um tipo proprio para chaves usadas no context.
// Isso evita colisoes com strings comuns usadas por outros pacotes ou middlewares.
type contextKey string

// claimsContextKey e a chave usada para guardar as claims dentro da requisicao.
// Como ela nao e exportada, apenas o pacote auth consegue acessar essa chave diretamente.
const claimsContextKey contextKey = "auth_claims"

// NewTokenManager cria um TokenManager pronto para gerar e validar access tokens.
// Essa funcao e chamada no main.go quando a API esta iniciando.
func NewTokenManager(secret string, ttl time.Duration) (*TokenManager, error) {
	// Confere se o segredo tem pelo menos 32 caracteres.
	if len(secret) < minSecretLength {
		// Se o segredo for fraco, a API nao deve subir.
		return nil, ErrJWTSecretTooShort
	}

	// Confere se o tempo de vida do token e maior que zero.
	if ttl <= 0 {
		// Se o token nao tiver um tempo valido, a API tambem nao deve subir.
		return nil, ErrJWTAccessTokenTTLInvalid
	}

	// Cria e devolve o TokenManager configurado.
	return &TokenManager{
		secret: []byte(secret), // Converte o segredo para bytes, formato esperado pela biblioteca JWT.
		ttl:    ttl,            // Guarda por quanto tempo cada access token vai valer.
		now:    time.Now,       // Usa o relogio real por padrao.
	}, nil
}

// Generate recebe um customer ja autenticado e cria um JWT assinado para ele.
// Essa funcao e usada pelo AuthController depois que register, login ou refresh da certo.
func (manager *TokenManager) Generate(customer model.Customer) (string, time.Time, error) {
	// Pega a hora atual em UTC para evitar confusao com fuso horario.
	now := manager.now().UTC()

	// Soma o TTL na hora atual para descobrir quando o access token vai expirar.
	expiresAt := now.Add(manager.ttl)

	// Monta os dados que serao colocados dentro do JWT.
	claims := tokenClaims{
		Email: customer.Email, // Coloca o email do customer no token.
		Name:  customer.Name,  // Coloca o nome do customer no token.
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   customer.ID.String(),          // Coloca o ID do customer no campo padrao "sub".
			IssuedAt:  jwt.NewNumericDate(now),       // Coloca a data de criacao no campo "iat".
			ExpiresAt: jwt.NewNumericDate(expiresAt), // Coloca a data de vencimento no campo "exp".
		},
	}

	// Cria um JWT novo usando HS256 e as claims montadas acima.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Assina o token com o segredo da aplicacao.
	// Depois da assinatura, qualquer alteracao no token faz a validacao falhar.
	signedToken, err := token.SignedString(manager.secret)

	// Confere se ocorreu algum erro durante a assinatura.
	if err != nil {
		// Devolve o erro original embrulhado com contexto para facilitar o diagnostico no servidor.
		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}

	// Devolve o JWT pronto, sua data de expiracao e nenhum erro.
	return signedToken, expiresAt, nil
}

// Validate recebe o texto de um JWT e confere se ele pode ser considerado confiavel.
// Ela valida assinatura, algoritmo, expiracao e campos obrigatorios.
func (manager *TokenManager) Validate(tokenString string) (Claims, error) {
	// Cria uma struct vazia onde a biblioteca colocara os dados lidos do token.
	parsedClaims := tokenClaims{}

	// ParseWithClaims le o token, valida a assinatura e preenche parsedClaims.
	token, err := jwt.ParseWithClaims(
		tokenString,   // Texto bruto do JWT recebido na requisicao.
		&parsedClaims, // Endereco da struct que recebera as claims extraidas.
		func(token *jwt.Token) (interface{}, error) {
			// Confere o algoritmo declarado dentro do token.
			// Isso protege contra tentativas de trocar o algoritmo esperado pela aplicacao.
			if token.Method.Alg() != jwtAlgorithm {
				// Se o algoritmo nao for HS256, o token e recusado.
				return nil, fmt.Errorf("algoritmo inesperado: %s", token.Method.Alg())
			}

			// O algoritmo esta correto; devolve o segredo para a biblioteca validar a assinatura.
			return manager.secret, nil
		},
		jwt.WithValidMethods([]string{jwtAlgorithm}), // Reforca que somente HS256 e permitido.
		jwt.WithExpirationRequired(),                 // Exige que o token tenha o campo "exp".
		jwt.WithIssuedAt(),                           // Valida o campo "iat", que informa quando o token foi criado.
		jwt.WithTimeFunc(func() time.Time {
			// Usa o relogio do TokenManager para comparar as datas do token.
			return manager.now().UTC()
		}),
	)

	// Se a biblioteca identificou que o token expirou, devolve um erro especifico.
	if errors.Is(err, jwt.ErrTokenExpired) {
		return Claims{}, ErrAccessTokenExpired
	}

	// Recusa qualquer outro erro, um token ausente ou um token marcado como invalido.
	if err != nil || token == nil || !token.Valid {
		return Claims{}, ErrAccessTokenInvalid
	}

	// Confere se Subject e um UUID valido.
	// Neste projeto, Subject deve representar o ID do customer autenticado.
	if _, err := uuid.Parse(parsedClaims.Subject); err != nil {
		return Claims{}, ErrAccessTokenInvalid
	}

	// Confere se as claims obrigatorias do projeto estao presentes.
	if parsedClaims.Email == "" || parsedClaims.IssuedAt == nil || parsedClaims.ExpiresAt == nil {
		return Claims{}, ErrAccessTokenInvalid
	}

	// Converte o formato interno tokenClaims para o formato Claims usado pelo restante da aplicacao.
	return Claims{
		Subject:   parsedClaims.Subject,          // ID do customer autenticado.
		Email:     parsedClaims.Email,            // Email do customer autenticado.
		Name:      parsedClaims.Name,             // Nome do customer autenticado.
		IssuedAt:  parsedClaims.IssuedAt.Unix(),  // Data de criacao convertida para timestamp Unix.
		ExpiresAt: parsedClaims.ExpiresAt.Unix(), // Data de expiracao convertida para timestamp Unix.
	}, nil
}

// Middleware roda antes dos controllers protegidos.
// Ele extrai e valida o JWT, guarda as claims no context e so depois libera a requisicao.
func (manager *TokenManager) Middleware(next http.Handler) http.Handler {
	// Transforma uma funcao comum em um http.Handler que o chi consegue usar como middleware.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Pega o header Authorization e tenta extrair o token depois da palavra Bearer.
		token, ok := bearerToken(r.Header.Get("Authorization"))

		// Se o header nao existe ou esta no formato errado, bloqueia a requisicao.
		if !ok {
			http.Error(w, ErrBearerTokenRequired.Error(), http.StatusUnauthorized)
			return
		}

		// Valida assinatura, algoritmo, expiracao e claims do token extraido.
		claims, err := manager.Validate(token)

		// Se o token expirou, responde 401 com a mensagem especifica de expiracao.
		if errors.Is(err, ErrAccessTokenExpired) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Se o token tem qualquer outro problema, responde 401 com a mensagem generica.
		// A resposta generica evita revelar detalhes uteis para quem esta atacando a API.
		if err != nil {
			http.Error(w, ErrAccessTokenInvalid.Error(), http.StatusUnauthorized)
			return
		}

		// Guarda as claims validadas no contexto desta requisicao.
		// Controllers e outros middlewares poderao descobrir quem fez a chamada.
		ctx := context.WithValue(r.Context(), claimsContextKey, claims)

		// Chama o proximo handler com uma copia da requisicao que contem o novo contexto.
		// Se esta linha nao for executada, a rota protegida nunca chega ao controller.
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext recupera os dados do usuario autenticado guardados pelo Middleware.
// Ela e usada por logs e controllers depois que o JWT ja foi validado.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	// Busca o valor salvo com claimsContextKey e tenta converte-lo para Claims.
	claims, ok := ctx.Value(claimsContextKey).(Claims)

	// Retorna as claims e um booleano que informa se o valor existia no formato esperado.
	return claims, ok
}

// bearerToken recebe o valor completo do header Authorization.
// Exemplo esperado: "Bearer eyJhbGciOi...".
func bearerToken(authorization string) (string, bool) {
	// Remove espacos das pontas e corta o texto no primeiro espaco.
	// "Bearer abc" vira scheme="Bearer", token="abc" e ok=true.
	scheme, token, ok := strings.Cut(strings.TrimSpace(authorization), " ")

	// Recusa se nao conseguiu separar as partes, se o esquema nao e Bearer ou se o token esta vazio.
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}

	// Retorna o token sem espacos extras e true para indicar sucesso.
	return strings.TrimSpace(token), true
}
