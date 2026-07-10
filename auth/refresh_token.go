package auth // Este arquivo faz parte do pacote auth, que cuida de autenticacao.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"cliente-api/model"

	"github.com/google/uuid"
)

const (
	// refreshTokenPrefix identifica visualmente o tipo do token.
	// O prefixo ajuda a reconhecer e validar o formato, mas nao e uma protecao criptografica.
	refreshTokenPrefix = "rt_"

	// refreshTokenEntropyBytes define quantos bytes aleatorios formam o segredo do token.
	// 32 bytes correspondem a 256 bits de entropia, o que torna o token impraticavel de adivinhar.
	refreshTokenEntropyBytes = 32
)

var (
	// ErrRefreshTokenIdleTTLInvalid aparece quando o prazo de inatividade e zero ou negativo.
	ErrRefreshTokenIdleTTLInvalid = errors.New("tempo de inatividade do refresh token deve ser positivo")

	// ErrRefreshTokenAbsoluteTTLInvalid aparece quando o prazo maximo da sessao e zero ou negativo.
	ErrRefreshTokenAbsoluteTTLInvalid = errors.New("tempo absoluto da sessao deve ser positivo")

	// ErrRefreshTokenLifetimeInvalid aparece quando o limite absoluto e menor que o limite de inatividade.
	// Essa configuracao nao faria sentido, pois toda sessao venceria antes do seu primeiro TTL de inatividade.
	ErrRefreshTokenLifetimeInvalid = errors.New("tempo absoluto da sessao deve ser maior ou igual ao tempo de inatividade")
)

// RefreshTokenManager e o componente responsavel por criar e interpretar refresh tokens.
// Diferente do access token JWT, o refresh token e opaco: o cliente nao consegue ler dados dentro dele.
type RefreshTokenManager struct {
	// idleTTL define quanto tempo o token atual pode ficar sem ser usado.
	// Cada rotacao renova esse prazo, sem ultrapassar o limite absoluto da familia.
	idleTTL time.Duration

	// absoluteTTL define por quanto tempo a familia inteira da sessao pode existir.
	// Esse prazo nasce no login/cadastro e nunca e prorrogado por uma rotacao.
	absoluteTTL time.Duration

	// now devolve a hora atual.
	// Guardar a funcao permite substituir o relogio nos testes e evitar sleeps desnecessarios.
	now func() time.Time

	// reader fornece bytes aleatorios seguros.
	// Em producao usamos crypto/rand.Reader; em testes podemos fornecer bytes previsiveis ou simular falhas.
	reader io.Reader
}

// NewRefreshTokenManager valida os tempos configurados e cria um gerenciador pronto para uso.
// Essa funcao e chamada no main.go durante a inicializacao da API.
func NewRefreshTokenManager(idleTTL, absoluteTTL time.Duration) (*RefreshTokenManager, error) {
	// O prazo de inatividade precisa ser maior que zero para o token ter uma validade coerente.
	if idleTTL <= 0 {
		return nil, ErrRefreshTokenIdleTTLInvalid
	}

	// A duracao total da sessao tambem precisa ser maior que zero.
	if absoluteTTL <= 0 {
		return nil, ErrRefreshTokenAbsoluteTTLInvalid
	}

	// O limite absoluto nao pode terminar antes do prazo de inatividade configurado.
	if absoluteTTL < idleTTL {
		return nil, ErrRefreshTokenLifetimeInvalid
	}

	// Devolve o gerenciador com o relogio e a fonte de aleatoriedade usados em producao.
	return &RefreshTokenManager{
		idleTTL:     idleTTL,     // Guarda o limite de inatividade, por exemplo 7 dias.
		absoluteTTL: absoluteTTL, // Guarda o limite total da sessao, por exemplo 30 dias.
		now:         time.Now,    // Usa o relogio real por padrao.
		reader:      rand.Reader, // Usa uma fonte criptograficamente segura de numeros aleatorios.
	}, nil
}

// Generate cria um refresh token bruto e os metadados que serao persistidos no banco.
// O valor bruto e entregue ao cliente uma unica vez; o banco recebe apenas o hash desse valor.
func (manager *RefreshTokenManager) Generate() (string, model.RefreshToken, error) {
	// Reserva um espaco de 32 bytes para receber o segredo aleatorio do token.
	randomBytes := make([]byte, refreshTokenEntropyBytes)

	// Preenche todos os 32 bytes usando a fonte segura configurada no manager.
	// ReadFull so considera sucesso quando o slice inteiro foi preenchido.
	if _, err := io.ReadFull(manager.reader, randomBytes); err != nil {
		// Sem aleatoriedade segura, nao podemos emitir uma credencial confiavel.
		return "", model.RefreshToken{}, fmt.Errorf("generate refresh token: %w", err)
	}

	// Converte os bytes para Base64 URL-safe sem padding.
	// O resultado usa apenas caracteres seguros para JSON, URLs e headers.
	rawToken := refreshTokenPrefix + base64.RawURLEncoding.EncodeToString(randomBytes)

	// Registra o momento exato de criacao em UTC para evitar diferencas de fuso horario.
	now := manager.now().UTC()

	// Calcula o limite absoluto da nova familia de sessao.
	// Em uma rotacao, o repository substitui este valor pelo limite da familia original.
	familyExpiresAt := now.Add(manager.absoluteTTL)

	// Calcula quando o token atual vence por inatividade.
	expiresAt := now.Add(manager.idleTTL)

	// Garante que o token individual nunca sobreviva ao limite absoluto da familia.
	if expiresAt.After(familyExpiresAt) {
		expiresAt = familyExpiresAt
	}

	// Cria um UUID v7 para identificar este registro no banco.
	// O UUID ajuda na organizacao dos registros; a seguranca do token vem dos 32 bytes aleatorios.
	tokenID, err := uuid.NewV7()
	if err != nil {
		return "", model.RefreshToken{}, fmt.Errorf("generate refresh token id: %w", err)
	}

	// Devolve duas representacoes do mesmo refresh token:
	// rawToken vai para o cliente, enquanto RefreshToken contem apenas o hash e metadados para o banco.
	return rawToken, model.RefreshToken{
		ID: tokenID, // Identificador do registro deste token.

		// Salva apenas o hash para que um vazamento do banco nao revele tokens utilizaveis.
		TokenHash: hashRefreshToken(rawToken),

		ExpiresAt:       expiresAt,       // Prazo de inatividade do token atual.
		FamilyExpiresAt: familyExpiresAt, // Prazo maximo da familia da sessao.
		CreatedAt:       now,             // Momento em que o token foi criado.
	}, nil
}

// Hash valida o formato de um refresh token recebido e calcula o hash usado na busca do banco.
// Essa funcao nao confirma se o token existe, expirou ou foi revogado; o repository faz essas verificacoes.
func (manager *RefreshTokenManager) Hash(rawToken string) ([]byte, error) {
	// Remove espacos acidentais no inicio e no fim do valor recebido.
	rawToken = strings.TrimSpace(rawToken)

	// Recusa valores que nao comecam com o prefixo usado pelos nossos refresh tokens.
	if !strings.HasPrefix(rawToken, refreshTokenPrefix) {
		return nil, model.ErrRefreshTokenInvalid
	}

	// Remove o prefixo e tenta recuperar os bytes aleatorios codificados em Base64 URL-safe.
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(rawToken, refreshTokenPrefix))

	// Recusa Base64 invalido e tokens com tamanho diferente dos 32 bytes esperados.
	// A verificacao de tamanho impede aceitar formatos menores, maiores ou incompletos.
	if err != nil || len(decoded) != refreshTokenEntropyBytes {
		return nil, model.ErrRefreshTokenInvalid
	}

	// O formato esta correto; devolve o hash que sera comparado com token_hash no banco.
	return hashRefreshToken(rawToken), nil
}

// hashRefreshToken transforma o token bruto em um resumo SHA-256 de 32 bytes.
// SHA-256 e adequado aqui porque o token ja possui 256 bits aleatorios; ele nao e uma senha escolhida por uma pessoa.
func hashRefreshToken(rawToken string) []byte {
	// Sum256 calcula o hash e devolve um array fixo de 32 bytes.
	digest := sha256.Sum256([]byte(rawToken))

	// Cria um slice porque o model e o driver do banco trabalham com []byte, nao com [32]byte.
	hash := make([]byte, len(digest))

	// Copia os bytes do array fixo para o slice que sera devolvido.
	copy(hash, digest[:])

	// Retorna somente o hash; o valor bruto nunca e persistido pelo servidor.
	return hash
}
