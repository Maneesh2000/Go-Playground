// constants.js — run status vocabulary shared across components.

export const TERMINAL = new Set([
  'success',
  'compile_error',
  'runtime_error',
  'time_limit_exceeded',
  'internal_error',
]);

export const STATUS_LABELS = {
  idle: 'Idle',
  queued: 'Queued',
  running: 'Running',
  success: 'Success',
  compile_error: 'Compile Error',
  runtime_error: 'Runtime Error',
  time_limit_exceeded: 'Time Limit',
  internal_error: 'Internal Error',
};
