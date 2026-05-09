import axios from 'axios';
import { useAuthStore } from '../stores/authStore';

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
});

api.interceptors.request.use(
  (config) => {
    const { token } = useAuthStore.getState();
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      useAuthStore.getState().logout();
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

export const authAPI = {
  login: (username: string, password: string) => api.post('/auth/login', { username, password }),
  register: (username: string, password: string) => api.post('/auth/register', { username, password }),
  refresh: (refreshToken: string) => api.post('/auth/refresh', { refresh_token: refreshToken }),
  me: () => api.get('/auth/me'),
};

export const hostAPI = {
  list: (params?: { status?: string; region?: string }) => api.get('/hosts', { params }),
  get: (id: string) => api.get(`/hosts/${id}`),
  create: (data: any) => api.post('/hosts', data),
  update: (id: string, data: any) => api.patch(`/hosts/${id}`, data),
  delete: (id: string) => api.delete(`/hosts/${id}`),
  sshConnect: (id: string) => api.post(`/hosts/${id}/ssh-connect`),
};

export const desktopAPI = {
  list: () => api.get('/desktops'),
  create: (data: any) => api.post('/desktops', data),
  terminate: (id: string) => api.delete(`/desktops/${id}`),
  deleteRecord: (id: string) => api.delete(`/desktops/${id}/record`),
  batchTerminate: (ids: string[]) => api.post('/desktops/batch/terminate', { ids }),
  batchDelete: (ids: string[]) => api.post('/desktops/batch/delete', { ids }),
};

export const fileAPI = {
  list: (desktopId: string, path: string = '.') =>
    api.get(`/desktops/${desktopId}/files`, { params: { path } }),

  upload: (desktopId: string, path: string, file: File, relativePath?: string) => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('path', path);
    if (relativePath) {
      formData.append('relativePath', relativePath);
    }
    return api.post(`/desktops/${desktopId}/upload`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
  },

  download: (desktopId: string, path: string) =>
    api.get(`/desktops/${desktopId}/download`, { params: { path }, responseType: 'blob' }),

  del: (desktopId: string, path: string) =>
    api.delete(`/desktops/${desktopId}/files`, { params: { path } }),

  mkdir: (desktopId: string, path: string) =>
    api.post(`/desktops/${desktopId}/mkdir`, { path }),
};

export const settingsAPI = {
  getLDAP: () => api.get('/settings/ldap'),
  updateLDAP: (data: any) => api.post('/settings/ldap', data),
  testLDAP: () => api.post('/settings/ldap/test'),
};

export const collaborationAPI = {
  listInvited: () => api.get('/collaborations/invited'),
  listMyInvites: () => api.get('/collaborations'),
  create: (data: any) => api.post('/collaborations', data),
  stop: (id: string) => api.delete(`/collaborations/${id}`),
};

export default api;
