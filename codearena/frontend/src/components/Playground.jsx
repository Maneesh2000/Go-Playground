// Playground — the single post-login view: Go editor on the left, live
// terminal on the right, draggable divider between them.
//
// Run lifecycle: POST /api/runs -> watch run_event frames over the WebSocket
// (status / chunk / done), with a 2s polling fallback on GET /api/runs/{id}
// that stops as soon as the socket delivers anything for the run.

import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../api.js';
import { TERMINAL } from '../constants.js';
import { useToast } from '../context/ToastContext.jsx';
import { wsEnsure, wsSubscribe } from '../ws.js';
import { Spinner, StatusPill, fmtDate } from './Bits.jsx';
import CodeEditor from './CodeEditor.jsx';
import Terminal from './Terminal.jsx';

const DRAFT_KEY = 'ca_playground_code';

const HELLO_WORLD = `package main

import "fmt"

func main() {
	fmt.Println("Hello, CodeArena!")
}
`;

const fmtSecs = (ms) => (ms == null ? '' : ` · ${(ms / 1000).toFixed(2)}s`);

export default function Playground() {
  const toast = useToast();
  const [entries, setEntries] = useState([]); // terminal scrollback segments
  const [running, setRunning] = useState(false);
  const [runStatus, setRunStatus] = useState('idle');
  const [leftWidth, setLeftWidth] = useState(52); // percent

  const splitRef = useRef(null);
  const editorRef = useRef(null);
  const idRef = useRef(0);
  // In-flight run: { id, pollTimer, statusEntryId, gotChunks } — a ref so the
  // WS/poll callbacks always see the current one.
  const activeRef = useRef(null);

  useEffect(() => { document.title = 'CodeArena — Go Playground'; }, []);

  /* ---------- terminal scrollback helpers ---------- */

  const nextId = () => ++idRef.current;

  // Append a full line (adds a newline terminator; also makes sure it starts
  // on a fresh line if the previous chunk didn't end with one).
  const pushLine = useCallback((kind, text, id = nextId()) => {
    setEntries((es) => {
      const prev = es[es.length - 1];
      const lead = prev && !prev.text.endsWith('\n') ? '\n' : '';
      return [...es, { id, kind, text: `${lead}${text}\n` }];
    });
    return id;
  }, []);

  const updateLine = useCallback((id, kind, text) => {
    setEntries((es) => es.map((e) => (e.id === id ? { ...e, kind, text: `${text}\n` } : e)));
  }, []);

  // Append raw program output verbatim; coalesce with the previous segment
  // when the stream (stdout/stderr) is the same.
  const appendChunk = useCallback((kind, data) => {
    if (!data) return;
    setEntries((es) => {
      const prev = es[es.length - 1];
      if (prev && prev.kind === kind) {
        return [...es.slice(0, -1), { ...prev, text: prev.text + data }];
      }
      return [...es, { id: nextId(), kind, text: data }];
    });
  }, []);

  /* ---------- run lifecycle ---------- */

  const markRunning = useCallback((a) => {
    setRunStatus('running');
    if (a && !a.sawRunning) {
      a.sawRunning = true;
      updateLine(a.statusEntryId, 'dim', 'running…');
    }
  }, [updateLine]);

  const finishRun = useCallback((p) => {
    const a = activeRef.current;
    if (!a) return;
    if (a.pollTimer) clearInterval(a.pollTimer);
    activeRef.current = null;
    setRunning(false);
    const status = p.status || 'internal_error';
    setRunStatus(status);

    switch (status) {
      case 'success':
        pushLine('ok', `✓ exited with code 0${fmtSecs(p.runtime_ms)}`);
        break;
      case 'compile_error':
        if (p.error) appendChunk('err', p.error.endsWith('\n') ? p.error : `${p.error}\n`);
        pushLine('fail', '✗ compilation failed');
        break;
      case 'runtime_error':
        if (p.error && !a.gotChunks) appendChunk('err', p.error.endsWith('\n') ? p.error : `${p.error}\n`);
        pushLine('fail', `✗ process exited with code ${p.exit_code ?? 1}${fmtSecs(p.runtime_ms)}`);
        break;
      case 'time_limit_exceeded':
        pushLine('fail', `✗ ${p.error || 'execution exceeded 10s and was killed'}`);
        break;
      default: // internal_error and anything unexpected
        pushLine('fail', `✗ ${p.error || 'internal error — something went wrong, please retry'}`);
        break;
    }
  }, [appendChunk, pushLine]);

  // Live updates over WebSocket; the first WS frame for the active run
  // cancels the polling fallback.
  useEffect(() => wsSubscribe((p) => {
    const a = activeRef.current;
    if (!a || p.run_id !== a.id) return;
    if (a.pollTimer) { clearInterval(a.pollTimer); a.pollTimer = null; }

    if (p.type === 'status') {
      if (p.status === 'running') markRunning(a);
    } else if (p.type === 'chunk') {
      a.gotChunks = true;
      markRunning(a); // output implies the program is running
      appendChunk(p.stream === 'stderr' ? 'err' : 'out', p.data || '');
    } else if (p.type === 'done') {
      finishRun(p);
    }
  }), [appendChunk, finishRun, markRunning]);

  // Abandon tracking when the view unmounts (e.g. logout mid-run).
  useEffect(() => () => {
    const a = activeRef.current;
    if (a && a.pollTimer) clearInterval(a.pollTimer);
    activeRef.current = null;
  }, []);

  const run = useCallback(async () => {
    if (activeRef.current) return;
    const code = editorRef.current ? editorRef.current.getValue() : '';
    if (!code.trim()) { toast('Your program is empty.'); return; }

    setRunning(true);
    setRunStatus('queued');
    pushLine('prompt', '$ go run main.go');
    const statusEntryId = pushLine('dim', 'queued…');

    try {
      const res = await api('/api/runs', {
        method: 'POST',
        body: JSON.stringify({ code, language: 'go' }),
      });
      // 2s polling fallback in case the WS is down; stops on the first WS frame.
      const pollTimer = setInterval(() => {
        const a = activeRef.current;
        if (!a || a.id !== res.id) return;
        api(`/api/runs/${res.id}`).then((r) => {
          if (!r || !activeRef.current || activeRef.current.id !== res.id) return;
          if (r.status === 'running') markRunning(activeRef.current);
          if (TERMINAL.has(r.status)) {
            // No streaming happened — print the recorded output in one go.
            if (!activeRef.current.gotChunks && r.output) appendChunk('out', r.output);
            finishRun(r);
          }
        }).catch(() => { /* transient poll failure — keep trying */ });
      }, 2000);
      activeRef.current = { id: res.id, pollTimer, statusEntryId, gotChunks: false, sawRunning: false };
      wsEnsure(); // make sure the socket is up for live chunks
    } catch (e) {
      updateLine(statusEntryId, 'fail', `✗ ${e.message}`);
      toast(e.message);
      setRunning(false);
      setRunStatus('idle');
    }
  }, [appendChunk, finishRun, markRunning, pushLine, toast, updateLine]);

  const clearTerminal = useCallback(() => setEntries([]), []);

  /* ---------- divider drag ---------- */

  function onDividerDown(e) {
    e.preventDefault();
    document.body.classList.add('dragging');
    const rect = splitRef.current.getBoundingClientRect();
    const onMove = (ev) => {
      const pct = ((ev.clientX - rect.left) / rect.width) * 100;
      setLeftWidth(Math.max(25, Math.min(75, pct)));
    };
    const onUp = () => {
      document.body.classList.remove('dragging');
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }

  /* ---------- render ---------- */

  return (
    <div className="split" ref={splitRef}>
      {/* LEFT: editor */}
      <section className="panel panel-left" style={{ width: `${leftWidth}%` }}>
        <div className="editor-header">
          <span className="editor-title">main.go</span>
          <span className="lang-badge">Go</span>
        </div>
        <CodeEditor ref={editorRef} storageKey={DRAFT_KEY} initial={HELLO_WORLD} onRun={run} />
        <div className="editor-footer">
          <button className="btn btn-run" onClick={run} disabled={running}>
            {running ? <><Spinner size="tiny" /> Running…</> : '▶ Run'}
          </button>
          <button className="btn" onClick={clearTerminal}>Clear terminal</button>
          <div className="nav-spacer" />
          <RecentRuns
            onLoad={(code) => {
              if (editorRef.current) editorRef.current.setValue(code);
              toast('Code loaded into the editor.', 'info');
            }}
          />
        </div>
      </section>

      {/* Draggable divider */}
      <div className="divider" onMouseDown={onDividerDown} title="Drag to resize" />

      {/* RIGHT: terminal */}
      <section className="panel panel-right">
        <Terminal entries={entries} status={runStatus} />
      </section>
    </div>
  );
}

/* ---------- Recent runs dropdown ---------- */

function RecentRuns({ onLoad }) {
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [runs, setRuns] = useState(null); // null = loading

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setRuns(null);
    api('/api/runs')
      .then((list) => { if (!cancelled) setRuns(Array.isArray(list) ? list : []); })
      .catch((e) => {
        if (!cancelled) { setRuns([]); toast(e.message); }
      });
    return () => { cancelled = true; };
  }, [open, toast]);

  async function pick(r) {
    setOpen(false);
    try {
      const detail = await api(`/api/runs/${r.id}`);
      const code = (detail && detail.code) || r.snippet;
      if (code) onLoad(code);
      else toast('No code stored for this run.');
    } catch (e) {
      toast(e.message);
    }
  }

  return (
    <div className="runs-dd">
      <button className="btn-ghost" onClick={() => setOpen((o) => !o)}>
        Recent runs {open ? '▴' : '▾'}
      </button>
      {open && (
        <>
          <div className="runs-backdrop" onClick={() => setOpen(false)} />
          <div className="runs-menu">
            {runs === null ? (
              <div className="runs-note"><Spinner size="small" /></div>
            ) : runs.length === 0 ? (
              <div className="runs-note">No runs yet.</div>
            ) : (
              runs.map((r) => (
                <button className="runs-item" key={r.id} onClick={() => pick(r)}>
                  <span className="runs-snippet">{(r.snippet || '').split('\n')[0] || `run ${r.id}`}</span>
                  <span className="runs-meta">
                    <StatusPill status={r.status} />
                    <span className="runs-date">{fmtDate(r.created_at)}</span>
                  </span>
                </button>
              ))
            )}
          </div>
        </>
      )}
    </div>
  );
}
