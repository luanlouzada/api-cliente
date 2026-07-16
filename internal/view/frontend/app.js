const sessionStorageKey = "cliente_api_session";
const sessionStorageVersion = 1;
const sessionLockName = "cliente-api-session";
const accessTokenRefreshMarginMilliseconds = 5000;
const refreshRetryBaseMilliseconds = 5000;
const refreshRetryMaximumMilliseconds = 60000;
const refreshTokenPattern = /^rt_[A-Za-z0-9_-]{43}$/;

const initialSession = readStoredSession() || emptySession();

// O estado em memória espelha o único registro persistido. Toda mutação da
// sessão passa por withSessionLock para preservar a ordem entre abas.
const state = {
  mode: "login",
  loading: false,
  token: initialSession.token,
  expiresAt: initialSession.expiresAt,
  refreshToken: initialSession.refreshToken,
  sessionExpiresAt: initialSession.sessionExpiresAt,
  customer: initialSession.customer,
  refreshRetryAt: initialSession.refreshRetryAt,
  refreshRetryCount: initialSession.refreshRetryCount,
};

// As referências são resolvidas uma vez porque a página não recria esses elementos.
const elements = {
  loginTab: document.querySelector("#loginTab"),
  registerTab: document.querySelector("#registerTab"),
  authForm: document.querySelector("#authForm"),
  nameField: document.querySelector("#nameField"),
  phoneField: document.querySelector("#phoneField"),
  authName: document.querySelector("#authName"),
  authPhone: document.querySelector("#authPhone"),
  authPassword: document.querySelector("#authPassword"),
  authTitle: document.querySelector("#authTitle"),
  authKicker: document.querySelector("#authKicker"),
  authError: document.querySelector("#authError"),
  authSubmitButton: document.querySelector("#authSubmitButton"),
  sessionPanel: document.querySelector("#sessionPanel"),
  customerName: document.querySelector("#customerName"),
  customerEmail: document.querySelector("#customerEmail"),
  customerID: document.querySelector("#customerID"),
  clearSessionButton: document.querySelector("#clearSessionButton"),
  copyTokenButton: document.querySelector("#copyTokenButton"),
  copyStatus: document.querySelector("#copyStatus"),
  tokenExpiresAt: document.querySelector("#tokenExpiresAt"),
  tokenBox: document.querySelector("#tokenBox"),
};

let sessionRefreshTimer = null;
let localRefreshRetryCount = 0;

/**
 * Cria a forma vazia e previsível da sessão usada quando não há credenciais válidas.
 * @returns {Object} Sessão sem cliente, tokens, expirações ou tentativa pendente.
 */
function emptySession() {
  return {
    token: "",
    expiresAt: "",
    refreshToken: "",
    sessionExpiresAt: "",
    customer: null,
    refreshRetryAt: "",
    refreshRetryCount: 0,
  };
}

/**
 * Valida e copia o registro persistido sem confiar diretamente no conteúdo do navegador.
 * Metadados opcionais de retry inválidos voltam ao estado inicial sem invalidar credenciais completas.
 * @param {unknown} candidate Valor decodificado do localStorage.
 * @returns {Object|null} Sessão normalizada ou null quando o registro estiver incompleto.
 */
function normalizeSessionRecord(candidate) {
  const expiresAtTimestamp =
    typeof candidate?.expiresAt === "string"
      ? new Date(candidate.expiresAt).getTime()
      : Number.NaN;
  const sessionExpiresAtTimestamp =
    typeof candidate?.sessionExpiresAt === "string"
      ? new Date(candidate.sessionExpiresAt).getTime()
      : Number.NaN;

  if (
    !candidate ||
    typeof candidate !== "object" ||
    candidate.version !== sessionStorageVersion ||
    typeof candidate.token !== "string" ||
    candidate.token === "" ||
    typeof candidate.expiresAt !== "string" ||
    candidate.expiresAt === "" ||
    !Number.isFinite(expiresAtTimestamp) ||
    typeof candidate.refreshToken !== "string" ||
    !refreshTokenPattern.test(candidate.refreshToken) ||
    typeof candidate.sessionExpiresAt !== "string" ||
    candidate.sessionExpiresAt === "" ||
    !Number.isFinite(sessionExpiresAtTimestamp) ||
    !candidate.customer ||
    typeof candidate.customer !== "object" ||
    Array.isArray(candidate.customer) ||
    typeof candidate.customer.id !== "string" ||
    candidate.customer.id === "" ||
    typeof candidate.customer.name !== "string" ||
    candidate.customer.name === "" ||
    typeof candidate.customer.email !== "string" ||
    candidate.customer.email === "" ||
    (candidate.customer.role !== "customer" && candidate.customer.role !== "admin")
  ) {
    return null;
  }

  const candidateRefreshRetryAt =
    typeof candidate.refreshRetryAt === "string" ? candidate.refreshRetryAt : "";
  const refreshRetryAt =
    candidateRefreshRetryAt !== "" &&
    Number.isFinite(new Date(candidateRefreshRetryAt).getTime())
      ? candidateRefreshRetryAt
      : "";
  const refreshRetryCount =
    Number.isInteger(candidate.refreshRetryCount) && candidate.refreshRetryCount >= 0
      ? candidate.refreshRetryCount
      : 0;

  return {
    version: sessionStorageVersion,
    token: candidate.token,
    expiresAt: candidate.expiresAt,
    refreshToken: candidate.refreshToken,
    sessionExpiresAt: candidate.sessionExpiresAt,
    customer: candidate.customer,
    refreshRetryAt,
    refreshRetryCount,
  };
}

/**
 * Lê o único registro de sessão e impede que JSON corrompido interrompa a inicialização.
 * @returns {Object|null} Sessão persistida válida ou null quando ela não puder ser usada.
 */
function readStoredSession() {
  try {
    const serializedSession = localStorage.getItem(sessionStorageKey);
    if (!serializedSession) {
      return null;
    }
    return normalizeSessionRecord(JSON.parse(serializedSession));
  } catch {
    return null;
  }
}

/**
 * Substitui todos os campos de sessão em memória como uma única unidade lógica.
 * @param {Object|null} session Sessão normalizada ou null para aplicar o estado vazio.
 */
function replaceSessionInMemory(session) {
  const nextSession = session || emptySession();
  state.token = nextSession.token;
  state.expiresAt = nextSession.expiresAt;
  state.refreshToken = nextSession.refreshToken;
  state.sessionExpiresAt = nextSession.sessionExpiresAt;
  state.customer = nextSession.customer;
  state.refreshRetryAt = nextSession.refreshRetryAt;
  state.refreshRetryCount = nextSession.refreshRetryCount;
}

/**
 * Atualiza o estado em memória com a versão atômica mais recente do localStorage.
 * Essa releitura ocorre dentro do Web Lock antes de usar ou substituir qualquer token.
 */
function syncSessionFromStorage() {
  replaceSessionInMemory(readStoredSession());
}

/**
 * Confirma se os campos mínimos de uma sessão estão presentes em memória.
 * @returns {boolean} Verdadeiro quando tokens, datas e cliente formam uma sessão completa.
 */
function hasCompleteSession() {
  return Boolean(
    state.token &&
      state.expiresAt &&
      state.refreshToken &&
      state.sessionExpiresAt &&
      state.customer,
  );
}

/**
 * Informa se uma data de expiração existe e já passou; datas inválidas são consideradas expiradas.
 * A ausência de valor não significa expiração porque a completude é verificada separadamente.
 * @param {string} value Data no formato aceito por Date.
 * @returns {boolean} Verdadeiro quando a data é inválida ou já foi alcançada.
 */
function isExpired(value) {
  if (!value) {
    return false;
  }
  const timestamp = new Date(value).getTime();
  return !Number.isFinite(timestamp) || timestamp <= Date.now();
}

/**
 * Decide se o token de acesso deve ser renovado, usando cinco segundos de margem contra latência.
 * @param {string} value Data de expiração do token de acesso.
 * @returns {boolean} Verdadeiro quando o token está ausente, inválido ou próximo de expirar.
 */
function shouldRefreshAccessToken(value) {
  if (!value) {
    return true;
  }
  const timestamp = new Date(value).getTime();
  return (
    !Number.isFinite(timestamp) ||
    timestamp <= Date.now() + accessTokenRefreshMarginMilliseconds
  );
}

/**
 * Calcula quanto falta para o retry compartilhado; valores ausentes ou vencidos retornam zero.
 * @returns {number} Quantidade não negativa de milissegundos até a próxima tentativa.
 */
function refreshRetryDelayRemaining() {
  if (!state.refreshRetryAt) {
    return 0;
  }
  const retryAt = new Date(state.refreshRetryAt).getTime();
  if (!Number.isFinite(retryAt)) {
    return 0;
  }
  return Math.max(0, retryAt - Date.now());
}

/**
 * Agenda uma única restauração, respeitando retry compartilhado ou expiração do access token.
 * @param {number|null} requestedDelay Atraso explícito ou null para calcular pelo estado atual.
 */
function scheduleSessionRefresh(requestedDelay = null) {
  if (sessionRefreshTimer !== null) {
    window.clearTimeout(sessionRefreshTimer);
    sessionRefreshTimer = null;
  }
  if (!hasCompleteSession()) {
    return;
  }

  let delay = requestedDelay;
  if (!Number.isFinite(delay) || delay === null) {
    const retryDelay = refreshRetryDelayRemaining();
    if (retryDelay > 0) {
      delay = retryDelay;
    } else {
      const expiresAt = new Date(state.expiresAt).getTime();
      delay = Number.isFinite(expiresAt)
        ? Math.max(0, expiresAt - Date.now() - accessTokenRefreshMarginMilliseconds)
        : 0;
    }
  }

  const sessionExpiresAt = new Date(state.sessionExpiresAt).getTime();
  const absoluteSessionDelay = Number.isFinite(sessionExpiresAt)
    ? Math.max(0, sessionExpiresAt - Date.now())
    : 0;
  const boundedDelay = Math.min(Math.max(0, delay), absoluteSessionDelay);
  sessionRefreshTimer = window.setTimeout(restoreSessionFromTimer, boundedDelay);
}

/**
 * Calcula o atraso exponencial de uma tentativa sem ultrapassar um minuto.
 * @param {number} retryCount Número da tentativa, começando em um.
 * @returns {number} Atraso da tentativa em milissegundos.
 */
function refreshRetryDelayForCount(retryCount) {
  const exponent = Math.min(Math.max(retryCount - 1, 0), 4);
  return Math.min(
    refreshRetryBaseMilliseconds * 2 ** exponent,
    refreshRetryMaximumMilliseconds,
  );
}

/**
 * Agenda nova aquisição do Web Lock apenas nesta aba, sem alterar a sessão compartilhada.
 * Essa alternativa também funciona quando o lock falha antes de a sessão poder ser relida.
 * @param {Error} error Falha ao adquirir ou executar a seção crítica de restauração.
 */
function scheduleLocalSessionRetry(error) {
  localRefreshRetryCount = Math.min(localRefreshRetryCount + 1, 32);
  const delay = refreshRetryDelayForCount(localRefreshRetryCount);
  scheduleSessionRefresh(delay);
  elements.copyStatus.textContent = transientRefreshMessage(error);
}

/** Dispara a restauração agendada sem deixar uma Promise rejeitada sem observador. */
function restoreSessionFromTimer() {
  void restoreSession();
}

/**
 * Informa se o navegador oferece exclusão mútua entre abas pela API Web Locks.
 * @returns {boolean} Verdadeiro quando uma requisição de lock pode ser realizada.
 */
function supportsSessionLock() {
  return Boolean(navigator.locks && typeof navigator.locks.request === "function");
}

/**
 * Executa uma operação de sessão sob exclusão mútua quando o navegador oferece Web Locks.
 * O fallback mantém a interface utilizável, mas operações de refresh recusam rotação sem o recurso.
 * @param {Function} callback Operação completa que lê, usa e eventualmente substitui a sessão.
 * @returns {Promise<*>} Resultado produzido pela operação protegida.
 */
async function withSessionLock(callback) {
  if (!supportsSessionLock()) {
    return callback();
  }
  return navigator.locks.request(sessionLockName, { mode: "exclusive" }, callback);
}

/**
 * Converte os campos bem-sucedidos de um formulário em um objeto simples para montar o DTO.
 * @param {HTMLFormElement} form Formulário de autenticação.
 * @returns {Object} Pares de nome e valor informados pelo usuário.
 */
function formJSON(form) {
  return Object.fromEntries(new FormData(form).entries());
}

/** Remove imediatamente a senha digitada para que ela não permaneça no DOM. */
function clearPasswordField() {
  elements.authPassword.value = "";
}

/**
 * Alterna a interface entre login e cadastro, inclusive requisitos e autocomplete dos campos.
 * @param {"login"|"register"} mode Modo que define endpoint, campos e textos apresentados.
 */
function setMode(mode) {
  state.mode = mode;
  const isLogin = mode === "login";

  elements.loginTab.classList.toggle("tab-active", isLogin);
  elements.registerTab.classList.toggle("tab-active", !isLogin);
  elements.loginTab.setAttribute("aria-pressed", String(isLogin));
  elements.registerTab.setAttribute("aria-pressed", String(!isLogin));
  elements.nameField.classList.toggle("hidden", isLogin);
  elements.phoneField.classList.toggle("hidden", isLogin);
  elements.authName.required = !isLogin;
  elements.authPhone.required = !isLogin;
  elements.authPassword.autocomplete = isLogin ? "current-password" : "new-password";
  elements.authTitle.textContent = isLogin ? "Entrar" : "Criar conta";
  elements.authKicker.textContent = isLogin ? "Conta existente" : "Novo cliente";
  elements.authSubmitButton.textContent = isLogin ? "Entrar" : "Criar conta";
  clearPasswordField();
  hideAuthError();
}

/**
 * Reflete uma requisição em andamento no estado e impede envios duplicados do formulário.
 * @param {boolean} isLoading Indica se a autenticação está sendo processada.
 */
function setLoading(isLoading) {
  state.loading = isLoading;
  elements.authSubmitButton.disabled = isLoading;
}

/**
 * Exibe uma mensagem de autenticação já preparada para o usuário pela API ou pelo navegador.
 * @param {string} message Mensagem que será apresentada no formulário.
 */
function showAuthError(message) {
  elements.authError.textContent = message;
  elements.authError.classList.remove("hidden");
}

/** Limpa e oculta o erro ao trocar de modo ou iniciar uma nova tentativa. */
function hideAuthError() {
  elements.authError.textContent = "";
  elements.authError.classList.add("hidden");
}

/**
 * Converte a resposta HTTP no formato interno, incluindo os metadados iniciais de retry.
 * @param {Object} data Resposta dos endpoints de cadastro, login ou refresh.
 * @returns {Object} Registro completo e validado que pode ser persistido atomicamente.
 * @throws {Error} Erro estável quando o backend não devolver uma sessão completa.
 */
function sessionRecordFromAuthentication(data) {
  const session = normalizeSessionRecord({
    version: sessionStorageVersion,
    token: data?.access_token,
    expiresAt: data?.expires_at,
    refreshToken: data?.refresh_token,
    sessionExpiresAt: data?.session_expires_at,
    customer: data?.customer,
    refreshRetryAt: "",
    refreshRetryCount: 0,
  });
  if (!session) {
    throw new Error("a API devolveu uma sessão incompleta");
  }
  return session;
}

/**
 * Captura o estado atual no mesmo formato versionado usado pelo localStorage.
 * @returns {Object|null} Registro completo ou null quando já não existe uma sessão utilizável.
 */
function sessionRecordFromState() {
  return normalizeSessionRecord({
    version: sessionStorageVersion,
    token: state.token,
    expiresAt: state.expiresAt,
    refreshToken: state.refreshToken,
    sessionExpiresAt: state.sessionExpiresAt,
    customer: state.customer,
    refreshRetryAt: state.refreshRetryAt,
    refreshRetryCount: state.refreshRetryCount,
  });
}

/**
 * Grava a sessão inteira com um único setItem e só então atualiza a memória.
 * O chamador deve executar esta função dentro de withSessionLock.
 * @param {Object} session Registro já normalizado que substituirá a sessão anterior.
 */
function writeSessionRecord(session) {
  const normalizedSession = normalizeSessionRecord(session);
  if (!normalizedSession) {
    throw new Error("não foi possível persistir uma sessão incompleta");
  }
  localStorage.setItem(sessionStorageKey, JSON.stringify(normalizedSession));
  replaceSessionInMemory(normalizedSession);
}

/**
 * Persiste atomicamente o novo par de tokens e o cliente devolvidos pela API.
 * O retry volta ao estado inicial porque uma autenticação ou renovação acabou de funcionar.
 * @param {Object} data Resposta dos endpoints de cadastro, login ou refresh.
 */
function saveSession(data) {
  try {
    writeSessionRecord(sessionRecordFromAuthentication(data));
  } catch (cause) {
    const error = sessionAdoptionError("não foi possível armazenar a nova sessão");
    error.cause = cause;
    throw error;
  }
  localRefreshRetryCount = 0;
  renderSession();
}

/**
 * Remove o único registro de credenciais e aplica o estado vazio em memória.
 * O chamador deve executar esta função dentro de withSessionLock.
 */
function clearSession() {
  localRefreshRetryCount = 0;
  replaceSessionInMemory(null);
  clearPasswordField();
  try {
    localStorage.removeItem(sessionStorageKey);
  } finally {
    renderSession();
  }
}

/**
 * Projeta o estado atual na interface e mantém o agendamento de renovação coerente.
 * Nenhuma decisão de autorização é tomada aqui; o frontend apenas apresenta a sessão recebida.
 * @param {boolean} shouldScheduleRefresh Define se esta renderização deve substituir o timer atual.
 */
function renderSession(shouldScheduleRefresh = true) {
  const isAuthenticated = hasCompleteSession();
  const accessTokenUsable =
    isAuthenticated && !shouldRefreshAccessToken(state.expiresAt);
  elements.sessionPanel.classList.toggle("hidden", !isAuthenticated);
  elements.copyTokenButton.disabled = !accessTokenUsable;

  if (!isAuthenticated) {
    elements.tokenBox.value = "";
    elements.copyStatus.textContent = "";
    if (shouldScheduleRefresh) {
      scheduleSessionRefresh();
    }
    return;
  }

  elements.customerName.textContent = state.customer.name;
  const roleLabel = state.customer.role === "admin" ? "administrador" : "cliente";
  elements.customerEmail.textContent = `${state.customer.email} · ${roleLabel}`;
  elements.customerID.textContent = `ID: ${state.customer.id}`;
  elements.tokenExpiresAt.textContent = state.expiresAt
    ? `Expira em ${new Date(state.expiresAt).toLocaleString()}`
    : "-";
  elements.tokenBox.value = accessTokenUsable ? state.token : "";
  elements.copyStatus.textContent = "";
  if (shouldScheduleRefresh) {
    scheduleSessionRefresh();
  }
}

/**
 * Cria um erro público que preserva o status HTTP para decisões posteriores de retry.
 * @param {string} message Mensagem segura devolvida pela API ou fallback local.
 * @param {number} status Código HTTP recebido do endpoint de autenticação.
 * @returns {Error} Erro enriquecido com a propriedade numérica status.
 */
function authenticationRequestError(message, status) {
  const error = new Error(message);
  error.name = "AuthenticationRequestError";
  error.status = status;
  return error;
}

/**
 * Cria um erro local definitivo quando uma resposta bem-sucedida não pode substituir a sessão antiga.
 * @param {string} message Explicação pública e estável da falha de adoção.
 * @returns {Error} Erro marcado para impedir retry com um refresh token possivelmente consumido.
 */
function sessionAdoptionError(message) {
  const error = new Error(message);
  error.name = "SessionAdoptionError";
  error.definitiveSession = true;
  return error;
}

/**
 * Envia uma operação de autenticação e normaliza respostas de sucesso ou erro da API.
 * Falhas HTTP preservam status; falhas de rede continuam sem status e são tratadas como transitórias.
 * @param {string} path Endpoint relativo de autenticação.
 * @param {Object} payload DTO que será serializado como JSON.
 * @returns {Promise<Object|null>} Corpo decodificado da resposta bem-sucedida.
 * @throws {Error} Mensagem pública e status HTTP, quando houver resposta do servidor.
 */
async function requestAuth(path, payload) {
  const response = await fetch(path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });

  const raw = await response.text();
  let body = null;
  if (raw) {
    try {
      body = JSON.parse(raw);
    } catch {
      body = raw;
    }
  }

  if (!response.ok) {
    const message =
      typeof body === "string"
        ? body
        : body?.error?.message || `HTTP ${response.status}`;
    throw authenticationRequestError(message, response.status);
  }

  return body;
}

/**
 * Distingue erros que não serão corrigidos ao repetir o mesmo refresh token.
 * Cancelamento, conflito, processamento antecipado e limite de taxa permanecem transitórios.
 * @param {Error} error Falha produzida por requestAuth ou pelo navegador.
 * @returns {boolean} Verdadeiro para erros HTTP definitivos de cliente.
 */
function isDefinitiveSessionError(error) {
  if (error?.definitiveSession === true) {
    return true;
  }
  const status = Number(error?.status);
  if (!Number.isInteger(status) || status < 400 || status >= 500) {
    return false;
  }
  return ![408, 409, 425, 429, 499].includes(status);
}

/**
 * Constrói a mensagem de indisponibilidade sem expor detalhes técnicos ou descartar a sessão.
 * @param {Error} error Falha transitória de rede ou resposta HTTP do servidor.
 * @returns {string} Texto curto para informar que outra tentativa foi agendada.
 */
function transientRefreshMessage(error) {
  const status = Number(error?.status);
  if (Number.isInteger(status) && status > 0) {
    return `Renovação indisponível (HTTP ${status}); nova tentativa agendada.`;
  }
  return "Renovação indisponível; nova tentativa agendada.";
}

/**
 * Registra no mesmo objeto de sessão um retry exponencial compartilhado entre abas.
 * A sessão e o refresh token permanecem intactos porque a falha não confirmou invalidez.
 * @param {Error} error Falha transitória que motivou o novo agendamento.
 */
function scheduleRefreshRetry(error) {
  const currentSession = sessionRecordFromState();
  if (!currentSession) {
    return;
  }

  const retryCount = Math.min(currentSession.refreshRetryCount + 1, 32);
  const delay = refreshRetryDelayForCount(retryCount);
  currentSession.refreshRetryCount = retryCount;
  currentSession.refreshRetryAt = new Date(Date.now() + delay).toISOString();
  try {
    writeSessionRecord(currentSession);
  } catch {
    // Se o armazenamento falhar, ao menos esta aba preserva tokens e espera antes de repetir.
    replaceSessionInMemory(currentSession);
  }
  scheduleSessionRefresh(delay);
  elements.copyStatus.textContent = transientRefreshMessage(error);
}

/** Altera a View para o formulário de login sem modificar a sessão. */
function handleLoginTabClick() {
  setMode("login");
}

/** Altera a View para o formulário de cadastro sem modificar a sessão. */
function handleRegisterTabClick() {
  setMode("register");
}

/**
 * Revoga a sessão mais recente e sempre cumpre a intenção explícita de saída local.
 * A função inteira, inclusive a chamada de rede, é executada pelo mesmo Web Lock.
 * @returns {Promise<void>} Conclusão da tentativa remota e da limpeza local.
 */
async function logoutCurrentSession() {
  syncSessionFromStorage();
  try {
    if (state.refreshToken) {
      await requestAuth("/auth/logout", { refresh_token: state.refreshToken });
    }
  } finally {
    clearSession();
  }
}

/**
 * Trata o botão Sair, impede cliques duplicados e serializa logout com as demais mutações.
 * @returns {Promise<void>} Conclusão da operação de interface.
 */
async function handleLogoutClick() {
  elements.clearSessionButton.disabled = true;
  clearPasswordField();
  try {
    await withSessionLock(logoutCurrentSession);
  } catch {
    // logoutCurrentSession já remove a cópia local no finally quando a rede falha.
  } finally {
    elements.clearSessionButton.disabled = false;
  }
}

/**
 * Reconcilia a sessão compartilhada e copia somente um access token ainda utilizável.
 * @returns {Promise<void>} Conclusão da renovação opcional e da tentativa de cópia.
 */
async function handleCopyTokenClick() {
  const sessionReady = await restoreSession();
  if (!sessionReady) {
    return;
  }
  if (!state.token || shouldRefreshAccessToken(state.expiresAt)) {
    return;
  }

  try {
    await navigator.clipboard.writeText(state.token);
    elements.copyStatus.textContent = "Token copiado.";
  } catch {
    elements.copyStatus.textContent = "Não foi possível copiar automaticamente.";
  }
}

/**
 * Executa login ou cadastro e substitui a sessão antes de liberar o Web Lock.
 * @param {string} path Endpoint escolhido pelo modo do formulário.
 * @param {Object} payload DTO de autenticação já montado.
 * @returns {Promise<void>} Conclusão da autenticação e persistência atômica.
 */
async function authenticateAndSaveSession(path, payload) {
  const data = await requestAuth(path, payload);
  saveSession(data);
}

/**
 * Valida e envia o formulário, mantendo rede e persistência sob o mesmo Web Lock.
 * A senha é removida do DOM tanto no sucesso quanto no erro.
 * @param {SubmitEvent} event Evento de envio do formulário de autenticação.
 * @returns {Promise<void>} Conclusão da tentativa de login ou cadastro.
 */
async function handleAuthenticationSubmit(event) {
  event.preventDefault();
  hideAuthError();

  if (state.loading || !elements.authForm.reportValidity()) {
    return;
  }

  const form = formJSON(elements.authForm);
  const path = state.mode === "login" ? "/auth/login" : "/auth/register";
  const payload =
    state.mode === "login"
      ? { email: form.email, password: form.password }
      : { name: form.name, email: form.email, phone: form.phone, password: form.password };

  try {
    setLoading(true);
    await withSessionLock(authenticateAndSaveSession.bind(null, path, payload));
  } catch (error) {
    showAuthError(error.message);
  } finally {
    clearPasswordField();
    setLoading(false);
  }
}

/**
 * Reconcilia e eventualmente renova a sessão enquanto o Web Lock ainda está retido.
 * @returns {Promise<boolean>} Verdadeiro somente quando o access token está pronto para uso.
 */
async function restoreSessionWithinLock() {
  localRefreshRetryCount = 0;
  syncSessionFromStorage();
  if (!hasCompleteSession() || isExpired(state.sessionExpiresAt)) {
    clearSession();
    return false;
  }

  if (!shouldRefreshAccessToken(state.expiresAt)) {
    renderSession();
    return true;
  }

  renderSession(false);
  const retryDelay = refreshRetryDelayRemaining();
  if (retryDelay > 0) {
    scheduleSessionRefresh(retryDelay);
    elements.copyStatus.textContent = "Renovação pendente; nova tentativa já está agendada.";
    return false;
  }

  if (!supportsSessionLock()) {
    scheduleLocalSessionRetry(new Error("Web Locks indisponível"));
    elements.copyStatus.textContent =
      "Este navegador não oferece coordenação segura para renovar a sessão.";
    return false;
  }

  try {
    const data = await requestAuth("/auth/refresh", { refresh_token: state.refreshToken });
    saveSession(data);
    return true;
  } catch (error) {
    if (isDefinitiveSessionError(error)) {
      clearSession();
      return false;
    }
    scheduleRefreshRetry(error);
    return false;
  }
}

/**
 * Obtém o Web Lock e delega toda leitura, decisão, rede e mutação à mesma seção crítica.
 * @returns {Promise<boolean>} Verdadeiro quando existe um access token atual utilizável.
 */
async function restoreSession() {
  try {
    return await withSessionLock(restoreSessionWithinLock);
  } catch (error) {
    renderSession(false);
    scheduleLocalSessionRetry(error);
    return false;
  }
}

/**
 * Reconcilia somente eventos produzidos pela chave atômica de sessão em outra aba.
 * @param {StorageEvent} event Evento de alteração do armazenamento compartilhado.
 */
function handleStorageChange(event) {
  if (event.key === sessionStorageKey || event.key === null) {
    void restoreSession();
  }
}

elements.loginTab.addEventListener("click", handleLoginTabClick);
elements.registerTab.addEventListener("click", handleRegisterTabClick);
elements.clearSessionButton.addEventListener("click", handleLogoutClick);
elements.copyTokenButton.addEventListener("click", handleCopyTokenClick);
elements.authForm.addEventListener("submit", handleAuthenticationSubmit);
window.addEventListener("storage", handleStorageChange);

setMode("login");
void restoreSession();
