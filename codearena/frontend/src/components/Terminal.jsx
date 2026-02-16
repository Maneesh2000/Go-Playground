// Terminal — terminal-style output panel. Renders a stream of typed text
// segments (prompt / dim status / stdout / stderr / result lines) in a black
// scrollback. Auto-scrolls to the bottom on new output unless the user has
// scrolled up to read history.

import { useEffect, useRef } from 'react';
import { StatusPill } from './Bits.jsx';

export default function Terminal({ entries, status }) {
  const bodyRef = useRef(null);
  const stickRef = useRef(true); // stick to bottom until the user scrolls up

  function onScroll() {
    const el = bodyRef.current;
    if (!el) return;
    stickRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
  }

  // After new output, follow the bottom — but never yank the scroll position
  // away from a user who scrolled up.
  useEffect(() => {
    const el = bodyRef.current;
    if (el && stickRef.current) el.scrollTop = el.scrollHeight;
  }, [entries]);

  return (
    <div className="terminal">
      <div className="term-header">
        <span className="term-title">Terminal</span>
        <StatusPill status={status} />
        <div className="nav-spacer" />
        <span className="term-hint">Ctrl/Cmd + Enter to run</span>
      </div>
      <div className="term-body" ref={bodyRef} onScroll={onScroll}>
        {entries.length === 0 ? (
          <div className="term-empty">Press Run to execute your code.</div>
        ) : (
          <pre className="term-pre">
            {entries.map((e) => (
              <span key={e.id} className={`t-${e.kind}`}>{e.text}</span>
            ))}
          </pre>
        )}
      </div>
    </div>
  );
}
