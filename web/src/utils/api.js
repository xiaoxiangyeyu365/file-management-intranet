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
  list: (folderId = 0, sortBy = 'name', sortOrder = 'asc') =>
    api.get('/files', { params: { folderId, sortBy, sortOrder } }),
  listPaged: (folderId = 0, page = 1, pageSize = 50, sortBy = 'name', sortOrder = 'asc') =>
    api.get('/files', { params: { folderId, page, pageSize, sortBy, sortOrder } }),
  search: (keyword, folderId = null, sort = 'relevance') =>
    api.get('/files/search', { params: { keyword, folderId, sort } }),
  lookup: (parentId, name) =>
    api.get('/files/lookup', { params: { parentId, name } }),
  get: (id) => api.get(`/files/${id}`),
  getAISummary: (id) => api.get(`/files/${id}/ai-summary`),
  regenerateSummary: (id) => api.post(`/files/${id}/ai-summary`),
  reindex: (id) => api.post(`/files/${id}/reindex`),
  rename: (id, name) => api.put(`/files/${id}`, { name }),
  delete: (id) => api.delete(`/files/${id}`),
  move: (fileIds, targetFolderId) =>
    api.patch('/files/move', { fileIds, targetFolderId }),
  downloadUrl: (id) => `${import.meta.env.VITE_API_BASE_URL || '/api'}/files/${id}/download`,
  previewUrl: (id) => `${import.meta.env.VITE_API_BASE_URL || '/api'}/files/${id}/preview`
}

// Folders API
export const folderAPI = {
  create: (parentId, name) => api.post('/folders', { parentId, name }),
  downloadUrl: (id) =>
    `${import.meta.env.VITE_API_BASE_URL || '/api'}/folders/${id}/download`,
  listTree: () => api.get('/folders/tree')
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
  updateQuota: (id, quota) => api.put(`/admin/users/${id}/quota`, { disk_quota: quota }),
  getSettings: () => api.get('/admin/settings'),
  updateSettings: (data) => api.put('/admin/settings', data),
  getIndexStats: () => api.get('/admin/ai/index-stats'),
  reindexAll: () => api.post('/admin/ai/reindex-all'),
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
  availableFiles: () => api.get('/chat/available-files'),
  createConversation: (data) => api.post('/chat/conversations', data),
  listConversations: (limit = 20, offset = 0) =>
    api.get('/chat/conversations', { params: { limit, offset } }),
  getConversation: (id) => api.get(`/chat/conversations/${id}`),
  deleteConversation: (id) => api.delete(`/chat/conversations/${id}`),
  addFile: (conversationId, fileId) =>
    api.post(`/chat/conversations/${conversationId}/add-file`, { fileId }),
  removeFile: (conversationId, fileId) =>
    api.delete(`/chat/conversations/${conversationId}/files/${fileId}`),
  ask: (conversationId, question, onToken, onDone, onError, signal) => {
    const token = localStorage.getItem('cloudbox_token')
    const baseURL = import.meta.env.VITE_API_BASE_URL || '/api'
    const url = `${baseURL}/chat/conversations/${conversationId}/ask?q=${encodeURIComponent(question)}`

    let cancelled = false

    fetch(url, {
      headers: {
        'Accept': 'text/event-stream',
        'Cache-Control': 'no-cache',
        'Authorization': token ? `Bearer ${token}` : ''
      },
      signal
    }).then(async (response) => {
      if (!response.ok) {
        const text = await response.text().catch(() => '')
        if (!cancelled) onError(`请求失败 (${response.status}): ${text}`)
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buf = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done || cancelled) break

        buf += decoder.decode(value, { stream: true })
        // Parse SSE lines
        const lines = buf.split('\n')
        buf = lines.pop() || '' // keep incomplete line in buffer

        for (const line of lines) {
          const trimmed = line.trim()
          if (trimmed === '' || !trimmed.startsWith('data: ')) continue
          const data = trimmed.slice(6)
          try {
            const parsed = JSON.parse(data)
            if (parsed.content !== undefined && onToken) {
              onToken(parsed.content)
            }
            if (parsed.messageId !== undefined) {
              if (onDone) onDone(parsed)
              cancelled = true
            }
            if (parsed.error) {
              if (onError) onError(parsed.error)
              cancelled = true
            }
          } catch (e) { /* skip malformed JSON */ }
        }
      }
    }).catch((err) => {
      if (cancelled || err.name === 'AbortError') return
      onError('请求失败: ' + err.message)
    })

    return { close: () => { cancelled = true } }
  },
  reindexFile: (id) => api.post(`/files/${id}/reindex`)
}

export default api