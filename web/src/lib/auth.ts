export interface User {
  id: string;
  username: string;
  role: string;
}

export interface LoginResult {
  ok: boolean;
  error?: string;
}

export async function login(username: string, password: string): Promise<LoginResult> {
  const res = await fetch("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
    credentials: "include",
  });
  return res.json();
}

export async function logout(): Promise<void> {
  await fetch("/api/auth/logout", {
    method: "POST",
    credentials: "include"
  });
}

export async function getMe(): Promise<User | null> {
  try {
    const res = await fetch("/api/auth/me", {
      credentials: "include"
    });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}

export async function changePassword(password: string): Promise<{ ok: boolean; error?: string }> {
  const res = await fetch("/api/auth/password", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password }),
    credentials: "include",
  });
  return res.json();
}

export async function getUsers(): Promise<User[]> {
  const res = await fetch("/api/users", {
    credentials: "include"
  });
  return res.json();
}

export async function createUser(username: string, password: string, role: string): Promise<User> {
  const res = await fetch("/api/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password, role }),
    credentials: "include",
  });
  return res.json();
}

export async function deleteUser(id: string): Promise<{ ok: boolean }> {
  const res = await fetch(`/api/users/${id}`, {
    method: "DELETE",
    credentials: "include"
  });
  return res.json();
}

export async function resetUserPassword(id: string, password: string): Promise<{ ok: boolean }> {
  const res = await fetch(`/api/users/${id}/password`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password }),
    credentials: "include",
  });
  return res.json();
}