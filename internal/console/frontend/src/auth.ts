const TOKEN_KEY = "graydraft_console_token";

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? "";
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

export function authHeaders(): Record<string, string> {
  const token = getToken();
  if (!token) return {};
  return { Authorization: `Bearer ${token}` };
}

export type LoginResult = {
  token: string;
  expires_at: string;
  subject: string;
};

export async function login(username: string, password: string): Promise<LoginResult> {
  const r = await fetch("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  const body = await r.json().catch(() => ({}));
  if (!r.ok) {
    throw new Error((body as { error?: string }).error ?? r.statusText);
  }
  return body as LoginResult;
}
