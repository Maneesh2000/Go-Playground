// App — hash router + auth gating.
// Routes: #/login, #/workspaces (default), #/workspace/<id>, #/playground.

import { useEffect } from 'react';
import { useAuth } from './context/AuthContext.jsx';
import { useHashRoute } from './hooks/useHashRoute.js';
import Login from './components/Login.jsx';
import Navbar from './components/Navbar.jsx';
import Playground from './components/Playground.jsx';
import Workspaces from './components/Workspaces.jsx';
import Workspace from './components/Workspace.jsx';
import { Spinner } from './components/Bits.jsx';

const KNOWN = ['#/workspaces', '#/playground'];
function isKnown(hash) {
  return KNOWN.includes(hash) || hash.startsWith('#/workspace/');
}

export default function App() {
  const { token, ready } = useAuth();
  const hash = useHashRoute();

  useEffect(() => {
    if (!ready) return;
    if (!token) {
      if (hash !== '#/login') location.hash = '#/login';
      return;
    }
    // Logged in: land on the workspaces list unless already on a known route.
    if (!isKnown(hash)) location.hash = '#/workspaces';
  }, [ready, token, hash]);

  if (!ready) {
    return <main id="view"><div className="page center-page"><Spinner /></div></main>;
  }
  if (!token) {
    return <main id="view"><Login /></main>;
  }

  let view;
  if (hash.startsWith('#/workspace/')) {
    view = <Workspace id={hash.slice('#/workspace/'.length)} />;
  } else if (hash === '#/playground') {
    view = <Playground />;
  } else {
    view = <Workspaces />;
  }

  return (
    <>
      <Navbar />
      <main id="view">{view}</main>
    </>
  );
}
