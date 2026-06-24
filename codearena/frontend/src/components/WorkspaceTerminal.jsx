// WorkspaceTerminal — xterm.js wired to the agent's `term` channel: keystrokes
// go to the PTY via termStdin, PTY output is written to the terminal.
import { useEffect, useRef } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

export default function WorkspaceTerminal({ conn, ready }) {
  const hostRef = useRef(null);
  const started = useRef(false);

  useEffect(() => {
    if (!conn || !ready || started.current || !hostRef.current) return;
    started.current = true;

    const term = new Terminal({
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: 13,
      cursorBlink: true,
      theme: { background: '#0b0e14' },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(hostRef.current);
    fit.fit();

    conn.termStart(term.cols, term.rows,
      (data) => term.write(data),
      (code) => term.write(`\r\n\x1b[90m[process exited: ${code}]\x1b[0m\r\n`),
    );
    const inputSub = term.onData((d) => conn.termStdin(d));

    const onResize = () => {
      try { fit.fit(); conn.termResize(term.cols, term.rows); } catch { /* ignore */ }
    };
    window.addEventListener('resize', onResize);
    const ro = new ResizeObserver(onResize);
    ro.observe(hostRef.current);

    return () => {
      window.removeEventListener('resize', onResize);
      ro.disconnect();
      inputSub.dispose();
      term.dispose();
      started.current = false;
    };
  }, [conn, ready]);

  return <div className="terminal-host" ref={hostRef} />;
}
