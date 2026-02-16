// App — hash router + auth gating. Routes: #/login, #/playground.

import { useEffect } from 'react';
import { useAuth } from './context/AuthContext.jsx';
import { useHashRoute } from './hooks/useHashRoute.js';
import Login from './components/Login.jsx';
import Navbar from './components/Navbar.jsx';
import Playground from './components/Playground.jsx';
import { Spinner } from './components/Bits.jsx';

export default function App() {
  const { token, ready } = useAuth();
  const hash = useHashRoute();

  // Redirect rules: no session -> #/login; session -> #/playground.
  useEffect(() => {
    if (!ready) return;
    if (!token) {
      if (hash !== '#/login') location.hash = '#/login';
      return;
    }
    if (hash !== '#/playground') location.hash = '#/playground';
  }, [ready, token, hash]);

  // Wait for the stored token to be validated before rendering a view.
  if (!ready) {
    return <main id="view"><div className="page center-page"><Spinner /></div></main>;
  }

  if (!token) {
    return <main id="view"><Login /></main>;
  }

  return (
    <>
      <Navbar />
      <main id="view"><Playground /></main>
    </>
  );
}
