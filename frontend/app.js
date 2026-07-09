const storageKeys = {
  token: "cliente_api_access_token",
  expiresAt: "cliente_api_expires_at",
  customer: "cliente_api_customer",
};

const state = {
  mode: "login",
  loading: false,
  token: localStorage.getItem(storageKeys.token) || "",
  expiresAt: localStorage.getItem(storageKeys.expiresAt) || "",
  customer: readStoredCustomer(),
};

const elements = {
  loginTab: document.querySelector("#loginTab"),
  registerTab: document.querySelector("#registerTab"),
  authForm: document.querySelector("#authForm"),
  nameField: document.querySelector("#nameField"),
  phoneField: document.querySelector("#phoneField"),
  authName: document.querySelector("#authName"),
  authPhone: document.querySelector("#authPhone"),
  authTitle: document.querySelector("#authTitle"),
  authKicker: document.querySelector("#authKicker"),
  authError: document.querySelector("#authError"),
  authSubmitButton: document.querySelector("#authSubmitButton"),
  sessionPanel: document.querySelector("#sessionPanel"),
  customerName: document.querySelector("#customerName"),
  customerEmail: document.querySelector("#customerEmail"),
  clearSessionButton: document.querySelector("#clearSessionButton"),
  copyTokenButton: document.querySelector("#copyTokenButton"),
  copyStatus: document.querySelector("#copyStatus"),
  tokenExpiresAt: document.querySelector("#tokenExpiresAt"),
  tokenBox: document.querySelector("#tokenBox"),
};

function readStoredCustomer() {
  try {
    return JSON.parse(localStorage.getItem(storageKeys.customer) || "null");
  } catch {
    return null;
  }
}

function formJSON(form) {
  return Object.fromEntries(new FormData(form).entries());
}

function setMode(mode) {
  state.mode = mode;
  const isLogin = mode === "login";

  elements.loginTab.classList.toggle("tab-active", isLogin);
  elements.registerTab.classList.toggle("tab-active", !isLogin);
  elements.nameField.classList.toggle("hidden", isLogin);
  elements.phoneField.classList.toggle("hidden", isLogin);
  elements.authName.required = !isLogin;
  elements.authPhone.required = !isLogin;
  elements.authTitle.textContent = isLogin ? "Entrar" : "Criar conta";
  elements.authKicker.textContent = isLogin ? "Conta existente" : "Novo customer";
  elements.authSubmitButton.textContent = isLogin ? "Entrar" : "Criar conta";
  hideAuthError();
}

function setLoading(isLoading) {
  state.loading = isLoading;
  elements.authSubmitButton.disabled = isLoading;
}

function showAuthError(message) {
  elements.authError.textContent = message;
  elements.authError.classList.remove("hidden");
}

function hideAuthError() {
  elements.authError.textContent = "";
  elements.authError.classList.add("hidden");
}

function saveSession(data) {
  state.token = data.access_token;
  state.expiresAt = data.expires_at;
  state.customer = data.customer;

  localStorage.setItem(storageKeys.token, state.token);
  localStorage.setItem(storageKeys.expiresAt, state.expiresAt);
  localStorage.setItem(storageKeys.customer, JSON.stringify(state.customer));
  renderSession();
}

function clearSession() {
  state.token = "";
  state.expiresAt = "";
  state.customer = null;

  localStorage.removeItem(storageKeys.token);
  localStorage.removeItem(storageKeys.expiresAt);
  localStorage.removeItem(storageKeys.customer);
  renderSession();
}

function renderSession() {
  const isAuthenticated = Boolean(state.token && state.customer);
  elements.sessionPanel.classList.toggle("hidden", !isAuthenticated);

  if (!isAuthenticated) {
    elements.tokenBox.value = "";
    elements.copyStatus.textContent = "";
    return;
  }

  elements.customerName.textContent = state.customer.name;
  elements.customerEmail.textContent = state.customer.email;
  elements.tokenExpiresAt.textContent = state.expiresAt ? `Expira em ${new Date(state.expiresAt).toLocaleString()}` : "-";
  elements.tokenBox.value = state.token;
  elements.copyStatus.textContent = "";
}

async function requestAuth(path, payload) {
  const response = await fetch(path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });

  const raw = await response.text();
  const body = raw ? JSON.parse(raw) : null;

  if (!response.ok) {
    throw new Error(typeof body === "string" ? body : `HTTP ${response.status}`);
  }

  return body;
}

elements.loginTab.addEventListener("click", () => setMode("login"));
elements.registerTab.addEventListener("click", () => setMode("register"));
elements.clearSessionButton.addEventListener("click", clearSession);
elements.copyTokenButton.addEventListener("click", async () => {
  if (!state.token) {
    return;
  }

  await navigator.clipboard.writeText(state.token);
  elements.copyStatus.textContent = "Token copiado.";
});

elements.authForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  hideAuthError();

  if (!elements.authForm.reportValidity()) {
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
    const data = await requestAuth(path, payload);
    saveSession(data);
  } catch (error) {
    showAuthError(error.message);
  } finally {
    setLoading(false);
  }
});

setMode("login");
renderSession();
