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
    folders: (state) => state.files.filter(f => f.is_folder),
    filesOnly: (state) => state.files.filter(f => !f.is_folder)
  },

  actions: {
    async fetchFiles(folderId = 0) {
      this.loading = true
      try {
        const files = await fileAPI.list(folderId)
        this.files = files
        this.currentFolder = folderId
        this.selectedIds = []
        this.searchKeyword = ''
        this.searchResults = []
        this.isSearching = false
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
        this.searchResults = await fileAPI.search(keyword, folderId, sort)
      } finally {
        this.loading = false
      }
    },

    async createFolder(parentId, name) {
      const newFolder = await folderAPI.create(parentId, name)
      if (parentId === this.currentFolder) {
        this.files.push(newFolder)
      }
      return newFolder
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

    downloadFile(fileId) {
      const url = fileAPI.downloadUrl(fileId)
      window.open(url, '_blank')
    },

    downloadFolder(folderId) {
      const url = folderAPI.downloadUrl(folderId)
      window.open(url, '_blank')
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