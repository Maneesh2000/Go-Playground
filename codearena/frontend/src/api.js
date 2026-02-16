// api.js — fetch wrapper: JSON, Bearer auth, {error} bodies, 401 -> auto logout.

let unauthorizedHandler = null;

export function setUnauthorizedHandler(fn) {
  unauthorizedHandler = fn;
}

export async function api(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) };
  const token = localStorage.getItem('ca_token');
  if (token) headers.Authorization = `Bearer ${token}`;

  let res;
  try {
    res = await fetch(path, { ...opts, headers });
  } catch {
    throw new Error('Network error — could not reach the server.');
  }

  // Expired/invalid session anywhere in the app -> back to login.
  if (res.status === 401 && token) {
    if (unauthorizedHandler) unauthorizedHandler();
    throw new Error('Session expired — please sign in again.');
  }

  let data = null;
  try { data = await res.json(); } catch { /* empty or non-JSON body */ }
  if (!res.ok) throw new Error((data && data.error) || `Request failed (${res.status})`);
  return data;
}
