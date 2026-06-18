// workspaceApi.js — REST helpers for the workspace lifecycle endpoints.
import { api } from './api.js';

export const listWorkspaces = () => api('/api/workspaces');
export const getWorkspace = (id) => api(`/api/workspaces/${id}`);
export const createWorkspace = (name) =>
  api('/api/workspaces', { method: 'POST', body: JSON.stringify({ name }) });
export const startWorkspace = (id) => api(`/api/workspaces/${id}/start`, { method: 'POST' });
export const stopWorkspace = (id) => api(`/api/workspaces/${id}/stop`, { method: 'POST' });
export const deleteWorkspace = (id) => api(`/api/workspaces/${id}`, { method: 'DELETE' });
