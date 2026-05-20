import { defineStore } from 'pinia'
import { fileAPI, folderAPI, trashAPI } from '@/utils/api'

export const useFilesStore = defineStore('files', {
  state: () => ({
    files: [],
    currentFolder: 0,
    selectedIds: [],
    viewMode: localStorage.getItem('cloudbox_viewMode') || 'grid',
    loading: false,
    searchKeyword: '',
    searchScope: null,
    searchSort: 'relevance',
    searchResults: [],
    isSearching: false,
    path: [{ id: 0, name: '根目录' }]
  }),

  getters: {
    folders: (state) => state.files.filter(f => f.isFolder),
    filesOnly: (state) => state.files.filter(f => !f.isFolder)
  },

  actions: {
    async fetchFiles(folderId = 0) {
      this.loading = true
      try {
        console.log('fetchFiles called with folderId:', folderId)
        const response = await fileAPI.list(folderId)
        console.log('fetchFiles response:', response)
        // Backend returns {code:0, data:{files:[]}}
        const data = response?.data || response
        console.log('fetchFiles data:', data)
        this.files = data.files || []
        this.currentFolder = folderId
        this.selectedIds = []
        this.searchKeyword = ''
        this.searchResults = []
        this.isSearching = false
      } catch (err) {
        console.error('fetchFiles error:', err)
      } finally {
        this.loading = false
      }
    },

    async searchFiles(keyword, folderId = null, sort = 'relevance') {
      if (!keyword.trim()) {
        this.searchResults = []
        this.isSearching = false
        return
      }

      this.loading = true
      this.isSearching = true
      this.searchKeyword = keyword
      this.searchScope = folderId
      this.searchSort = sort

      try {
        const response = await fileAPI.search(keyword, folderId, sort)
        const data = response?.data || response
        this.searchResults = data.files || data || []
      } finally {
        this.loading = false
      }
    },

    async createFolder(parentId, name) {
      const response = await folderAPI.create(parentId, name)
      const data = response?.data || response
      if (parentId === this.currentFolder) {
        this.files.push(data)
      }
      return data
    },

    async checkFileExists(name, parentId = null) {
      const targetFolder = parentId !== null ? parentId : this.currentFolder
      try {
        const response = await fileAPI.lookup(targetFolder, name)
        return response?.data || response
      } catch {
        return null
      }
    },

    async renameFile(fileId, newName) {
      await fileAPI.rename(fileId, newName)
      const file = this.files.find(f => f.id === fileId)
      if (file) file.name = newName
      const searchFile = this.searchResults.find(f => f.id === fileId)
      if (searchFile) searchFile.name = newName
    },

    async deleteFile(fileId) {
      await fileAPI.delete(fileId)
      this.files = this.files.filter(f => f.id !== fileId)
      this.searchResults = this.searchResults.filter(f => f.id !== fileId)
      this.selectedIds = this.selectedIds.filter(id => id !== fileId)
    },

    async moveFiles(fileIds, targetFolderId) {
      await fileAPI.move(fileIds, targetFolderId)
      this.files = this.files.filter(f => !fileIds.includes(f.id))
      this.selectedIds = []
      if (targetFolderId === this.currentFolder) {
        await this.fetchFiles(this.currentFolder)
      }
    },

    async downloadFile(fileId) {
      const token = localStorage.getItem('cloudbox_token')
      console.log('Download - token from localStorage:', token ? 'exists (' + token.length + ' chars)' : 'NULL')
      if (!token) {
        console.error('No token found, please login first')
        return
      }

      // window.open triggers browser download via Content-Disposition: attachment
      // Works for cross-origin and large files (no in-memory blob).
      const downloadUrl = `/api/files/${fileId}/download?token=${encodeURIComponent(token)}`
      window.open(downloadUrl, '_blank')
    },

    async downloadFolder(folderId) {
      const token = localStorage.getItem('cloudbox_token')
      const url = folderAPI.downloadUrl(folderId)
      const response = await fetch(url, {
        headers: { Authorization: `Bearer ${token}` }
      })
      const contentType = response.headers.get('content-type')

      // Check if response is an error JSON
      if (contentType && contentType.includes('application/json')) {
        const data = await response.json()
        if (data.code !== 0) {
          console.error('Download failed:', data.message)
          return
        }
      }

      if (response.ok) {
        const blob = await response.blob()
        const disposition = response.headers.get('Content-Disposition')
        let filename = `folder-${folderId}.zip`
        if (disposition) {
          // RFC 5987 format: filename*=UTF-8''name
          const parts = disposition.split("'")
          if (parts.length >= 3) {
            filename = decodeURIComponent(parts[2])
          } else {
            const match = disposition.match(/filename="?([^;"']+)/)
            if (match) filename = match[1]
          }
        }
        const a = document.createElement('a')
        a.href = URL.createObjectURL(blob)
        a.download = filename
        a.click()
        URL.revokeObjectURL(a.href)
      }
    },

    toggleViewMode() {
      this.viewMode = this.viewMode === 'grid' ? 'list' : 'grid'
      localStorage.setItem('cloudbox_viewMode', this.viewMode)
    },

    setSelected(ids) {
      this.selectedIds = ids
    },

    updatePath(path) {
      this.path = path
    },

    async navigateToFolder(folder) {
      await this.fetchFiles(folder.id)
      const idx = this.path.findIndex(p => p.id === folder.id)
      if (idx >= 0) {
        this.path = this.path.slice(0, idx + 1)
      } else {
        this.path.push({ id: folder.id, name: folder.name })
      }
    },

    async navigateToRoot() {
      await this.fetchFiles(0)
      this.path = [{ id: 0, name: '根目录' }]
    }
  }
})