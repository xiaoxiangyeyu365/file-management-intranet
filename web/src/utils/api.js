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
    api.post('/upload/init', { md5, name, parentId, size }),
  uploadChunk: (uploadId, index, chunk) => {
    const formData = new FormData()
    formData.append('chunk', chunk)
    return api.put(`/upload/${uploadId}/chunk/${index}`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
  },
  progress: (uploadId) => api.get(`/upload/${uploadId}/progress`),
  complete: (uploadId, name, parentId) =>
    api.post(`/upload/${uploadId}/complete`, { name, parentId }),
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
  get: (id) => api.get(`/preview/${id}`)
}

export default api