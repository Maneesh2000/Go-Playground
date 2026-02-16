// Navbar — logo, current user + logout.

import { useAuth } from '../context/AuthContext.jsx';

export default function Navbar() {
  const { user, logout } = useAuth();
  return (
    <header id="navbar">
      <a className="nav-logo" href="#/playground">
        <span className="braces">{'{ }'}</span> Code<b>Arena</b>
      </a>
      <span className="nav-tagline">Go Playground</span>
      <div className="nav-spacer" />
      <span className="nav-user">{(user && user.username) || ''}</span>
      <button className="btn-ghost" onClick={logout}>Log out</button>
    </header>
  );
}
