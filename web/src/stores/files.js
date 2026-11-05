import { defineStore } from 'pinia'
import { fileAPI, folderAPI, trashAPI } from '@/utils/api'

export const useFilesStore = defineStore('files', {
  state: () => ({
    files: [],
    currentFolder: 0,
    selectedIds: [],
    lastSelectedId: null,
    viewMode: localStorage.getItem('cloudbox_viewMode') || 'grid',
    loading: false,
    searchKeyword: '',
    searchScope: null,
    searchSort: 'relevance',
    searchResults: [],
    isSearching: false,
    path: [{ id: 0, name: '根目录' }],
    sortBy: ['name', 'size', 'updatedAt'].includes(localStorage.getItem('cloudbox_sortBy'))
      ? localStorage.getItem('cloudbox_sortBy')
      : 'name',
    sortOrder: localStorage.getItem('cloudbox_sortOrder') === 'desc' ? 'desc' : 'asc',
    // Pagination
    pagination: {
      enabled: false,
      page: 1,
      pageSize: 50,
      total: 0
    }
  }),

  getters: {
    folders: (state) => state.files.filter(f => f.isFolder),
    filesOnly: (state) => state.files.filter(f => !f.isFolder),
    // The server returns rows already ordered by the active sort (folders
    // first). Sorting here would only reorder the current page and disagree
    // with server-side pagination, so this is intentionally a pass-through.
    sortedFiles: (state) => state.files
  },

  actions: {
    async fetchFiles(folderId = 0) {
      this.loading = true
      try {
        const response = await fileAPI.listPaged(folderId, 1, this.pagination.pageSize, this.sortBy, this.sortOrder)
        const data = response?.data || response
        this.files = data.files || []
        this.currentFolder = folderId
        this.selectedIds = []
        this.searchKeyword = ''
        this.searchResults = []
        this.isSearching = false
        this.pagination = {
          enabled: true,
          page: data.page || 1,
          pageSize: this.pagination.pageSize,
          total: data.total || 0
        }
      } catch (err) {
        console.error('fetchFiles error:', err)
      } finally {
        this.loading = false
      }
    },

    async fetchPage(page, options = {}) {
      if (!options.silent) this.loading = true
      try {
        const response = await fileAPI.listPaged(this.currentFolder, page, this.pagination.pageSize, this.sortBy, this.sortOrder)
        const data = response?.data || response
        this.files = data.files || []
        if (!options.keepSelection) this.selectedIds = []
        this.pagination.page = data.page || page
        this.pagination.total = data.total || 0
        this.pagination.enabled = true
      } catch (err) {
        console.error('fetchPage error:', err)
      } finally {
        if (!options.silent) this.loading = false
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
        // Keep server-side ordering intact instead of appending to the page.
        await this.fetchPage(this.pagination.page, { silent: true })
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
      // Name is a sort key; re-fetch so the row moves to its new position.
      if (!this.isSearching) {
        await this.fetchPage(this.pagination.page, { silent: true, keepSelection: true })
      }
    },

    async deleteFile(fileId) {
      await fileAPI.delete(fileId)
      this.files = this.files.filter(f => f.id !== fileId)
      this.searchResults = this.searchResults.filter(f => f.id !== fileId)
      this.selectedIds = this.selectedIds.filter(id => id !== fileId)
      if (this.lastSelectedId === fileId) this.lastSelectedId = null
      if (this.pagination.enabled) {
        this.pagination.total = Math.max(0, this.pagination.total - 1)
      }
      this.notifyStorageChanged()
    },

    async moveFiles(fileIds, targetFolderId) {
      await fileAPI.move(fileIds, targetFolderId)
      this.files = this.files.filter(f => !fileIds.includes(f.id))
      this.selectedIds = this.selectedIds.filter(id => !fileIds.includes(id))
      if (fileIds.includes(this.lastSelectedId)) this.lastSelectedId = null
      if (targetFolderId === this.currentFolder) {
        await this.fetchFiles(this.currentFolder)
      }
    },

    async downloadFile(fileId) {
      const token = localStorage.getItem('cloudbox_token')
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
      if (!token) return
      // Use window.open to stream download — avoids loading entire ZIP into browser memory
      const baseURL = import.meta.env.VITE_API_BASE_URL || '/api'
      const downloadUrl = `${baseURL}/folders/${folderId}/download?token=${encodeURIComponent(token)}`
      window.open(downloadUrl, '_blank')
    },

    async batchDownload() {
      const token = localStorage.getItem('cloudbox_token')
      if (!token) {
        console.error('No token found, please login first')
        return
      }

      const ids = this.selectedIds.join(',')
      // Use window.open to stream download — avoids loading entire ZIP into browser memory
      const baseURL = import.meta.env.VITE_API_BASE_URL || '/api'
      const downloadUrl = `${baseURL}/files/download?ids=${ids}&token=${encodeURIComponent(token)}`
      window.open(downloadUrl, '_blank')
    },

    toggleViewMode() {
      this.viewMode = this.viewMode === 'grid' ? 'list' : 'grid'
      localStorage.setItem('cloudbox_viewMode', this.viewMode)
    },

    setSelected(ids) {
      this.selectedIds = ids
      if (ids.length > 0) {
        this.lastSelectedId = ids[ids.length - 1]
      } else {
        this.lastSelectedId = null
      }
    },

    // Select a range between the last selected file and the target file
    selectRange(allFiles, targetId) {
      if (!this.lastSelectedId) {
        // No anchor, just toggle
        this.toggleSelect(targetId)
        return
      }
      const ids = allFiles.map(f => f.id)
      const anchorIdx = ids.indexOf(this.lastSelectedId)
      const targetIdx = ids.indexOf(targetId)
      if (anchorIdx === -1 || targetIdx === -1) {
        this.toggleSelect(targetId)
        return
      }
      const start = Math.min(anchorIdx, targetIdx)
      const end = Math.max(anchorIdx, targetIdx)
      const rangeIds = ids.slice(start, end + 1)
      // Add range to current selection
      const merged = new Set([...this.selectedIds, ...rangeIds])
      this.selectedIds = [...merged]
    },

    toggleSelect(id) {
      const idx = this.selectedIds.indexOf(id)
      if (idx >= 0) {
        this.selectedIds = this.selectedIds.filter(i => i !== id)
      } else {
        this.selectedIds.push(id)
      }
      this.lastSelectedId = id
    },

    setSort(sortBy, sortOrder) {
      if (this.sortBy === sortBy && this.sortOrder === sortOrder) return
      this.sortBy = sortBy
      this.sortOrder = sortOrder
      localStorage.setItem('cloudbox_sortBy', sortBy)
      localStorage.setItem('cloudbox_sortOrder', sortOrder)
      this.pagination.page = 1
      // Server-side ordering is the source of truth, so refetch the first page.
      if (!this.isSearching) {
        this.fetchPage(1, { silent: true })
      }
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
      this.pagination.page = 1
    },

    async navigateToRoot() {
      await this.fetchFiles(0)
      this.path = [{ id: 0, name: '根目录' }]
    },

    // Notify sidebar to refresh storage usage display
    notifyStorageChanged() {
      window.dispatchEvent(new Event('cloudbox:storage-changed'))
    }
  }
})