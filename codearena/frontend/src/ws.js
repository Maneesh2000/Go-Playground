// ws.js — singleton WebSocket with exponential-backoff reconnect while logged in.
// Components subscribe to "run_event" payloads via wsSubscribe.

let ws = null;
let timer = null;
let backoff = 1000;
let token = null;

const listeners = new Set();

/** Set (or clear) the auth token; connects or disconnects accordingly. */
export function wsSetToken(t) {
  token = t;
  if (t) connect();
  else disconnect();
}

/** Make sure the socket is up (e.g. right before starting a run). */
export function wsEnsure() {
  if (token) connect();
}

/** Subscribe to run_event payloads. Returns an unsubscribe fn. */
export function wsSubscribe(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

function connect() {
  if (!token) return;
  if (ws && (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN)) return;
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  try {
    ws = new WebSocket(`${proto}://${location.host}/ws?token=${encodeURIComponent(token)}`);
  } catch {
    schedule();
    return;
  }
  ws.onopen = () => { backoff = 1000; };
  ws.onmessage = (ev) => {
    let msg;
    try { msg = JSON.parse(ev.data); } catch { return; }
    if (msg && msg.type === 'run_event' && msg.payload) {
      listeners.forEach((fn) => fn(msg.payload));
    }
  };
  ws.onclose = () => { ws = null; schedule(); };
  ws.onerror = () => { /* onclose follows and schedules a reconnect */ };
}

function schedule() {
  if (!token) return;
  clearTimeout(timer);
  timer = setTimeout(connect, backoff);
  backoff = Math.min(backoff * 2, 15000);
}

function disconnect() {
  clearTimeout(timer);
  timer = null;
  if (ws) {
    ws.onclose = null;
    try { ws.close(); } catch { /* ignore */ }
    ws = null;
  }
}
