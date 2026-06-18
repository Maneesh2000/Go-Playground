// agentConn.js — WebSocket client for a workspace's in-pod agent, proxied by the
// control plane at /ws/workspace/<id>?token=<jwt>. It speaks the same protocol
// the Go agent serves: request/response `fs` ops (correlated by id) and a
// streaming `term` channel. Terminal payloads are UTF-8 strings, base64 on the wire.

function b64encode(s) {
  return btoa(String.fromCharCode(...new TextEncoder().encode(s)));
}
function b64decode(b) {
  return new TextDecoder().decode(Uint8Array.from(atob(b || ''), (c) => c.charCodeAt(0)));
}

export class AgentConn {
  constructor(id, token) {
    this.id = id;
    this.token = token;
    this.ws = null;
    this.nextId = 1;
    this.pending = new Map(); // fs request id -> {resolve, reject}
    this.term = { data: null, exit: null };
    this.onOpen = null;
    this.onClose = null;
    this.onError = null;
  }

  connect() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${location.host}/ws/workspace/${this.id}?token=${encodeURIComponent(this.token)}`;
    this.ws = new WebSocket(url);
    this.ws.onopen = () => this.onOpen && this.onOpen();
    this.ws.onclose = () => this.onClose && this.onClose();
    this.ws.onerror = (e) => this.onError && this.onError(e);
    this.ws.onmessage = (ev) => this._onMessage(ev.data);
  }

  _onMessage(raw) {
    let m;
    try { m = JSON.parse(raw); } catch { return; }
    if (m.ch === 'fs' && m.op === 'result') {
      const p = this.pending.get(m.id);
      if (p) {
        this.pending.delete(m.id);
        if (m.err) p.reject(new Error(m.err));
        else p.resolve(m);
      }
    } else if (m.ch === 'term') {
      if (m.op === 'data' && this.term.data) this.term.data(b64decode(m.data_b64));
      else if (m.op === 'exit' && this.term.exit) this.term.exit(m.code, m.err);
    }
  }

  _send(m) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(m));
  }

  _fs(op, extra) {
    const id = this.nextId++;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this._send({ ch: 'fs', op, id, ...extra });
      setTimeout(() => {
        if (this.pending.delete(id)) reject(new Error(`fs ${op} timed out`));
      }, 15000);
    });
  }

  list(path) { return this._fs('list', { path }).then((r) => r.entries || []); }
  read(path) { return this._fs('read', { path }).then((r) => b64decode(r.content_b64)); }
  write(path, content) { return this._fs('write', { path, content_b64: b64encode(content) }); }
  mkdir(path) { return this._fs('mkdir', { path }); }
  remove(path) { return this._fs('delete', { path }); }
  rename(path, to) { return this._fs('rename', { path, to }); }

  termStart(cols, rows, onData, onExit) {
    this.term.data = onData;
    this.term.exit = onExit;
    this._send({ ch: 'term', op: 'start', cols, rows });
  }
  termStdin(str) { this._send({ ch: 'term', op: 'stdin', data_b64: b64encode(str) }); }
  termResize(cols, rows) { this._send({ ch: 'term', op: 'resize', cols, rows }); }

  close() {
    if (this.ws) {
      this.ws.onclose = null;
      try { this.ws.close(); } catch { /* ignore */ }
      this.ws = null;
    }
    this.pending.clear();
  }
}
