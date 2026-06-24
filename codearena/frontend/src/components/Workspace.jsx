// Workspace — the IDE view. Owns the AgentConn to the workspace pod and wires
// the file tree, Monaco tabs, and terminal together. Route: #/workspace/<id>.
import { useEffect, useRef, useState, useCallback } from 'react';
import { useAuth } from '../context/AuthContext.jsx';
import { AgentConn } from '../agentConn.js';
import { getWorkspace, stopWorkspace } from '../workspaceApi.js';
import { navigate } from '../hooks/useHashRoute.js';
import FileTree from './FileTree.jsx';
import EditorPane from './EditorPane.jsx';
import WorkspaceTerminal from './WorkspaceTerminal.jsx';

export default function Workspace({ id }) {
  const { token } = useAuth();
  const [meta, setMeta] = useState(null);
  const [status, setStatus] = useState('connecting'); // connecting | ready | disconnected
  const [files, setFiles] = useState([]); // [{path, content, dirty}]
  const [activePath, setActivePath] = useState(null);
  const [treeKey, setTreeKey] = useState(0);
  const connRef = useRef(null);

  // Load metadata (name, preview url).
  useEffect(() => { getWorkspace(id).then(setMeta).catch(() => {}); }, [id]);

  // Open the agent connection (control plane resumes the pod if hibernated).
  useEffect(() => {
    if (!token) return;
    const conn = new AgentConn(id, token);
    connRef.current = conn;
    conn.onOpen = () => setStatus('ready');
    conn.onClose = () => setStatus('disconnected');
    conn.onError = () => setStatus('disconnected');
    conn.connect();
    return () => { conn.close(); connRef.current = null; };
  }, [id, token]);

  const openFile = useCallback(async (path) => {
    setFiles((prev) => {
      if (prev.some((f) => f.path === path)) return prev;
      return prev; // added below after read
    });
    if (files.some((f) => f.path === path)) { setActivePath(path); return; }
    try {
      const content = await connRef.current.read(path);
      setFiles((prev) => prev.some((f) => f.path === path) ? prev : [...prev, { path, content, dirty: false }]);
      setActivePath(path);
    } catch (e) { console.error('open', path, e); }
  }, [files]);

  const change = useCallback((path, content) => {
    setFiles((prev) => prev.map((f) => f.path === path ? { ...f, content, dirty: true } : f));
  }, []);

  const save = useCallback(async (path) => {
    const f = files.find((x) => x.path === path);
    if (!f) return;
    try {
      await connRef.current.write(path, f.content);
      setFiles((prev) => prev.map((x) => x.path === path ? { ...x, dirty: false } : x));
      setTreeKey((k) => k + 1); // a save may have created a new file
    } catch (e) { console.error('save', path, e); }
  }, [files]);

  const close = useCallback((path) => {
    setFiles((prev) => prev.filter((f) => f.path !== path));
    setActivePath((cur) => (cur === path ? null : cur));
  }, []);

  const newFile = useCallback(async () => {
    const name = prompt('New file path (relative to workspace root):');
    if (!name) return;
    try {
      await connRef.current.write(name, '');
      setFiles((prev) => [...prev, { path: name, content: '', dirty: false }]);
      setActivePath(name);
      setTreeKey((k) => k + 1);
    } catch (e) { console.error('new file', e); }
  }, []);

  const doStop = useCallback(async () => {
    try { await stopWorkspace(id); } catch { /* ignore */ }
    navigate('#/workspaces');
  }, [id]);

  return (
    <div className="ws-ide">
      <div className="ws-topbar">
        <button className="link-btn" onClick={() => navigate('#/workspaces')}>← Workspaces</button>
        <span className="ws-title">{meta ? meta.name : id}</span>
        <span className={`ws-status ${status}`}>{status}</span>
        <div className="ws-actions">
          <button onClick={newFile}>New file</button>
          {meta && meta.preview_url && (
            <button onClick={() => window.open(meta.preview_url, '_blank')}>Open preview ↗</button>
          )}
          <button onClick={doStop}>Stop</button>
        </div>
      </div>

      <div className="ws-body">
        <aside className="ws-sidebar">
          {status === 'ready' && connRef.current ? (
            <FileTree conn={connRef.current} refreshKey={treeKey} onOpenFile={openFile} />
          ) : (
            <div className="ws-note">{status === 'connecting' ? 'Starting workspace…' : 'Disconnected'}</div>
          )}
        </aside>
        <section className="ws-main">
          <EditorPane
            files={files}
            activePath={activePath}
            onSelect={setActivePath}
            onClose={close}
            onChange={change}
            onSave={save}
          />
          <div className="ws-terminal">
            <div className="pane-head">TERMINAL</div>
            <WorkspaceTerminal conn={connRef.current} ready={status === 'ready'} />
          </div>
        </section>
      </div>
    </div>
  );
}
