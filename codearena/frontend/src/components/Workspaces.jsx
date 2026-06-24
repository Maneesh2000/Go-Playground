// Workspaces — list/create/open/delete the user's persistent projects.
import { useEffect, useState, useCallback } from 'react';
import { listWorkspaces, createWorkspace, deleteWorkspace } from '../workspaceApi.js';
import { navigate } from '../hooks/useHashRoute.js';

export default function Workspaces() {
  const [items, setItems] = useState([]);
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  const reload = useCallback(() => {
    listWorkspaces().then(setItems).catch((e) => setError(e.message));
  }, []);
  useEffect(() => { reload(); }, [reload]);

  const create = useCallback(async (e) => {
    e.preventDefault();
    if (!name.trim()) return;
    setBusy(true); setError(null);
    try {
      const ws = await createWorkspace(name.trim());
      setName('');
      navigate(`#/workspace/${ws.id}`);
    } catch (err) { setError(err.message); } finally { setBusy(false); }
  }, [name]);

  const remove = useCallback(async (id) => {
    if (!confirm('Delete this workspace and its files? This cannot be undone.')) return;
    try { await deleteWorkspace(id); reload(); } catch (err) { setError(err.message); }
  }, [reload]);

  return (
    <div className="page workspaces-page">
      <h1>Workspaces</h1>
      <p className="muted">Persistent projects you edit in the browser and run on the cluster.</p>

      <form className="ws-create" onSubmit={create}>
        <input
          placeholder="new workspace name…"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={busy}
        />
        <button type="submit" disabled={busy || !name.trim()}>{busy ? 'Creating…' : 'Create'}</button>
      </form>
      {error && <div className="err">{error}</div>}

      <ul className="ws-list">
        {items.length === 0 && <li className="muted">No workspaces yet — create one above.</li>}
        {items.map((w) => (
          <li key={w.id} className="ws-item">
            <button className="ws-open" onClick={() => navigate(`#/workspace/${w.id}`)}>
              <span className="ws-name">{w.name}</span>
              <span className={`ws-badge ${w.status}`}>{w.status}</span>
            </button>
            <button className="ws-del" onClick={() => remove(w.id)} title="Delete">🗑</button>
          </li>
        ))}
      </ul>
    </div>
  );
}
