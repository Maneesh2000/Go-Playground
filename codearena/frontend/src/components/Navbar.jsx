// Navbar — logo, current user + logout.

import { useAuth } from '../context/AuthContext.jsx';

export default function Navbar() {
  const { user, logout } = useAuth();
  return (
    <header id="navbar">
      <a className="nav-logo" href="#/workspaces">
        <span className="braces">{'{ }'}</span> Code<b>Arena</b>
      </a>
      <nav className="nav-links">
        <a href="#/workspaces">Workspaces</a>
        <a href="#/playground">Playground</a>
      </nav>
      <div className="nav-spacer" />
      <span className="nav-user">{(user && user.username) || ''}</span>
      <button className="btn-ghost" onClick={logout}>Log out</button>
    </header>
  );
}
