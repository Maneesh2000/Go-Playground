// Login — centered auth card with sign-in / register toggle and demo credentials box.

import { useEffect, useState } from 'react';
import { api } from '../api.js';
import { useAuth } from '../context/AuthContext.jsx';
import { useToast } from '../context/ToastContext.jsx';
import { navigate } from '../hooks/useHashRoute.js';
import { Spinner } from './Bits.jsx';

export default function Login() {
  const { login } = useAuth();
  const toast = useToast();
  const [mode, setMode] = useState('login'); // 'login' | 'register'
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => { document.title = 'CodeArena — Sign in'; }, []);

  const isRegister = mode === 'register';

  async function onSubmit(e) {
    e.preventDefault();
    if (busy) return;
    if (!username.trim() || !password) { toast('Please fill in username and password.'); return; }
    if (isRegister && !email.trim()) { toast('Please provide an email address.'); return; }

    const payload = isRegister
      ? { username: username.trim(), email: email.trim(), password }
      : { username: username.trim(), password };

    setBusy(true);
    try {
      const data = await api(isRegister ? '/api/register' : '/api/login', {
        method: 'POST',
        body: JSON.stringify(payload),
      });
      if (data && data.token) {
        login(data.token, data.user);
        navigate('#/playground');
      } else {
        // register returns 201 without a session — drop into sign-in
        toast('Account created — please sign in.', 'success');
        setMode('login');
        setBusy(false);
      }
    } catch (err) {
      toast(err.message);
      setBusy(false);
    }
  }

  return (
    <div className="auth-wrap">
      <div className="auth-card">
        <div className="auth-logo"><span className="braces">{'{ }'}</span> Code<b>Arena</b></div>
        <div className="auth-sub">Write Go. Run it. Watch the output live.</div>

        <form onSubmit={onSubmit} noValidate>
          <label className="field">
            Username
            <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
          </label>
          {isRegister && (
            <label className="field">
              Email
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" />
            </label>
          )}
          <label className="field">
            Password
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" />
          </label>
          <button type="submit" className="btn btn-primary btn-block" disabled={busy}>
            {busy ? <Spinner size="tiny" /> : (isRegister ? 'Create account' : 'Sign in')}
          </button>
        </form>

        <div className="auth-toggle">
          {isRegister ? 'Already have an account? ' : 'Don’t have an account? '}
          <a
            href="#/login"
            onClick={(e) => { e.preventDefault(); setMode(isRegister ? 'login' : 'register'); }}
          >
            {isRegister ? 'Sign in' : 'Register'}
          </a>
        </div>

        <div className="demo-box">
          Demo credentials — username: <code>demo</code> · password: <code>demo123</code>
        </div>
      </div>
    </div>
  );
}
