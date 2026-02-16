// useHashRoute — minimal hash-based routing (#/login, #/playground).

import { useEffect, useState } from 'react';

export function useHashRoute() {
  const [hash, setHash] = useState(() => location.hash);
  useEffect(() => {
    const onChange = () => setHash(location.hash);
    window.addEventListener('hashchange', onChange);
    return () => window.removeEventListener('hashchange', onChange);
  }, []);
  return hash;
}

export function navigate(to) {
  location.hash = to;
}
