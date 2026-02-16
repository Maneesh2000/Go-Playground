// AuthContext — token + user state, persisted in localStorage.
// Validates the stored token against /api/me on boot and keeps the
// WebSocket connection in sync with the session.

import { createContext, useCallback, useContext, useEffect, useState } from 'react';
import { api, setUnauthorizedHandler } from '../api.js';
import { wsSetToken } from '../ws.js';

const AuthContext = createContext(null);

function readStoredUser() {
  try { return JSON.parse(localStorage.getItem('ca_user') || 'null'); } catch { return null; }
}

export function AuthProvider({ children }) {
  const [token, setToken] = useState(() => localStorage.getItem('ca_token'));
  const [user, setUser] = useState(readStoredUser);
  // ready = stored token has been validated (or there was none)
  const [ready, setReady] = useState(() => !localStorage.getItem('ca_token'));

  const logout = useCallback(() => {
    localStorage.removeItem('ca_token');
    localStorage.removeItem('ca_user');
    wsSetToken(null);
    setToken(null);
    setUser(null);
  }, []);

  const login = useCallback((tok, usr) => {
    localStorage.setItem('ca_token', tok);
    localStorage.setItem('ca_user', JSON.stringify(usr || null));
    setToken(tok);
    setUser(usr || null);
    wsSetToken(tok);
  }, []);

  // Any 401 from the API layer ends the session.
  useEffect(() => {
    setUnauthorizedHandler(logout);
    return () => setUnauthorizedHandler(null);
  }, [logout]);

  // Validate the stored token once on boot.
  useEffect(() => {
    const stored = localStorage.getItem('ca_token');
    if (!stored) { setReady(true); return; }
    let cancelled = false;
    api('/api/me')
      .then((me) => {
        if (cancelled) return;
        const u = me && me.user ? me.user : me;
        setUser(u);
        localStorage.setItem('ca_user', JSON.stringify(u));
        wsSetToken(stored);
      })
      .catch(() => {
        // 401 already triggered logout; on a network hiccup proceed
        // optimistically with the stored session.
        if (!cancelled && localStorage.getItem('ca_token')) wsSetToken(stored);
      })
      .finally(() => { if (!cancelled) setReady(true); });
    return () => { cancelled = true; };
  }, []); // mount only

  return (
    <AuthContext.Provider value={{ token, user, ready, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
