import { defineStore } from 'pinia'
import { shareAPI } from '../utils/api'

export const useSharesStore = defineStore('shares', {
  state: () => ({
    myShares: [],
    fileShares: [],
    loading: false,
  }),
  actions: {
    async createShare(fileId, { password, expiresIn, maxDownloads } = {}) {
      const res = await shareAPI.create({ fileId, password: password || '', expiresIn: expiresIn || 0, maxDownloads: maxDownloads || 0 })
      return res.data
    },
    async fetchMyShares() {
      this.loading = true
      try {
        const res = await shareAPI.listMine()
        this.myShares = res.data || []
      } finally {
        this.loading = false
      }
    },
    async fetchFileShares(fileId) {
      const res = await shareAPI.listFile(fileId)
      this.fileShares = res.data || []
    },
    async revokeShare(id) {
      await shareAPI.revoke(id)
      this.myShares = this.myShares.filter(s => s.id !== id)
    },
    async getShareInfo(token) {
      const res = await shareAPI.getInfo(token)
      return res.data
    },
    async verifyShare(token, password = '') {
      const res = await shareAPI.verify(token, password)
      return res.data
    },
    getDownloadUrl(token, credential) {
      return shareAPI.downloadUrl(token, credential)
    },
  },
})
