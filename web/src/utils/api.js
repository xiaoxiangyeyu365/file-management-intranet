import axios from 'axios'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// Request interceptor: add Authorization header
api.interceptors.request.use(
  config => {
    const authStore = useAuthStore()
    if (authStore.token) {
      config.headers.Authorization = `Bearer ${authStore.token}`
    }
    return config
  },
  error => Promise.reject(error)
)

// Response interceptor: handle 401
api.interceptors.response.use(
  response => response.data,
  error => {
    if (error.response?.status === 401) {
      const authStore = useAuthStore()
      authStore.token = null
      authStore.user = null
      router.push('/login')
    }
    return Promise.reject(error)
  }
)

// Auth API
export const authAPI = {
  login: (username, password) => api.post('/auth/login', { username, password }),
  register: (username, password) => api.post('/auth/register', { username, password }),
  changePassword: (oldPassword, newPassword) =>
    api.post('/auth/password', { oldPassword, newPassword }),
  logout: () => api.post('/auth/logout'),
  profile: () => api.get('/auth/profile')
}

// Files API
export const fileAPI = {
  list: (folderId = 0) => api.get('/files', { params: { folderId } }),
  search: (keyword, folderId = null, sort = 'relevance') =>
    api.get('/files/search', { params: { keyword, folderId, sort } }),
  lookup: (parentId, name) =>
    api.get('/files/lookup', { params: { parentId, name } }),
  get: (id) => api.get(`/files/${id}`),
  rename: (id, name) => api.put(`/files/${id}`, { name }),
  delete: (id) => api.delete(`/files/${id}`),
  move: (fileIds, targetFolderId) =>
    api.patch('/files/move', { fileIds, targetFolderId }),
  downloadUrl: (id) => `${import.meta.env.VITE_API_BASE_URL || '/api'}/files/${id}/download`
}

// Folders API
export const folderAPI = {
  create: (parentId, name) => api.post('/folders', { parentId, name }),
  downloadUrl: (id) =>
    `${import.meta.env.VITE_API_BASE_URL || '/api'}/folders/${id}/download`
}

// Upload API
export const uploadAPI = {
  init: (md5, name, parentId, size) =>
    api.post('/upload/init', { md5, fileName: name, targetFolderId: parentId, fileSize: size }),
  uploadChunk: (uploadId, index, chunk) => {
    const formData = new FormData()
    formData.append('chunk', chunk)
    // Get token directly from localStorage to ensure it's included
    const token = localStorage.getItem('cloudbox_token')
    return api.put(`/upload/${uploadId}/chunk/${index}`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
        'Authorization': token ? `Bearer ${token}` : ''
      }
    })
  },
  progress: (uploadId) => api.get(`/upload/${uploadId}/progress`),
  complete: (uploadId, name, parentId, md5, size) =>
    api.post(`/upload/${uploadId}/complete`, { fileName: name, targetFolderId: parentId, md5, fileSize: size }),
  cancel: (uploadId) => api.delete(`/upload/${uploadId}`)
}

// Trash API
export const trashAPI = {
  list: () => api.get('/trash'),
  restore: (id) => api.post(`/trash/${id}/restore`),
  delete: (id) => api.delete(`/trash/${id}`),
  empty: () => api.delete('/trash')
}

// Preview API
export const previewAPI = {
  get: (id) => api.get(`/files/${id}/metadata`)
}

// Clipboard API
export const clipboardAPI = {
  list: () => api.get('/clipboard'),
  create: (content) => api.post('/clipboard', { content }),
  createWithDevice: (content, deviceName) =>
    api.post('/clipboard', { content, deviceName }),
  togglePin: (id, pinned) => api.patch(`/clipboard/${id}/pin`, { pinned }),
  delete: (id) => api.delete(`/clipboard/${id}`),
  clear: (onlyUnpinned = true) => api.delete('/clipboard', {
    params: { onlyUnpinned: onlyUnpinned.toString() }
  })
}

// Admin API
export const adminAPI = {
  listUsers: (status = '') => api.get('/admin/users', { params: { status } }),
  createUser: (username, password, role = 'user') =>
    api.post('/admin/users', { username, password, role }),
  updateUser: (id, data) => api.put(`/admin/users/${id}`, data),
  resetPassword: (id, newPassword) =>
    api.put(`/admin/users/${id}/password`, { newPassword }),
  deleteUser: (id) => api.delete(`/admin/users/${id}`),
  listAuditLogs: (params) => api.get('/admin/audit-logs', { params })
}

// Share API
export const shareAPI = {
  // Public (no auth header needed)
  getInfo: (token) => api.get(`/s/${token}`),
  verify: (token, password = '') => api.post(`/s/${token}/verify`, { password }),
  downloadUrl: (token, credential) => `${import.meta.env.VITE_API_BASE_URL || '/api'}/s/${token}/download?t=${encodeURIComponent(credential)}`,

  // Authenticated
  create: (data) => api.post('/shares', data),
  listFile: (fileId) => api.get('/shares', { params: { fileId } }),
  listMine: () => api.get('/shares/mine'),
  revoke: (id) => api.delete(`/shares/${id}`),
}

// Storage API
export const storageAPI = {
  getUsage: () => api.get('/storage/usage'),
}

// Chat API (RAG)
export const chatAPI = {
  createConversation: (data) => api.post('/chat/conversations', data),
  listConversations: (limit = 20, offset = 0) =>
    api.get('/chat/conversations', { params: { limit, offset } }),
  getConversation: (id) => api.get(`/chat/conversations/${id}`),
  deleteConversation: (id) => api.delete(`/chat/conversations/${id}`),
  addFile: (conversationId, fileId) =>
    api.post(`/chat/conversations/${conversationId}/add-file`, { fileId }),
  ask: async (conversationId, question) => {
    const token = localStorage.getItem('cloudbox_token')
    const baseURL = import.meta.env.VITE_API_BASE_URL || '/api'
    const resp = await fetch(`${baseURL}/chat/conversations/${conversationId}/ask`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': token ? `Bearer ${token}` : ''
      },
      body: JSON.stringify({ question })
    })
    return resp // Return raw Response for SSE streaming
  },
  reindexFile: (id) => api.post(`/files/${id}/reindex`)
}

export default api