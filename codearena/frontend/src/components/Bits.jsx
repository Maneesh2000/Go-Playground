// Bits — small shared presentational pieces: status pill, spinner, date fmt.

import { STATUS_LABELS } from '../constants.js';

export function StatusPill({ status }) {
  return <span className={`pill status-${status}`}>{STATUS_LABELS[status] || status}</span>;
}

export function Spinner({ size }) {
  return <div className={'spinner' + (size ? ` ${size}` : '')} />;
}

export function fmtDate(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}
