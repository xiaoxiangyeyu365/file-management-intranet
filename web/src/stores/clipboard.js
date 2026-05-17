import { defineStore } from 'pinia'
import { clipboardAPI } from '@/utils/api'
import { ElMessage } from 'element-plus'

const MAX_CONTENT_SIZE = 10240

export const useClipboardStore = defineStore('clipboard', {
  state: () => ({
    records: [],
    loading: false,
    pollingInterval: null
  }),

  getters: {
    pinnedRecords: (state) => state.records.filter(r => r.pinned),
    unpinnedRecords: (state) => state.records.filter(r => !r.pinned)
  },

  actions: {
    isNew(record) {
      const createdTime = new Date(record.createdAt).getTime()
      return Date.now() - createdTime < 5000
    },

    async fetchRecords() {
      this.loading = true
      try {
        const response = await clipboardAPI.list()
        const data = response?.data || response
        this.records = data.records || []
      } catch (err) {
        console.error('Failed to fetch clipboard records:', err)
      } finally {
        this.loading = false
      }
    },

    async createRecord(content) {
      if (!content || content.trim() === '') {
        ElMessage.error('请输入内容')
        return null
      }
      if (content.length > MAX_CONTENT_SIZE) {
        ElMessage.error('内容过长（最多 10240 字符）')
        return null
      }

      try {
        const response = await clipboardAPI.create(content)
        const data = response?.data || response
        this.records.unshift(data)
        if (this.records.length > 50) {
          this.records = this.records.slice(0, 50)
        }
        ElMessage.success('已保存到云剪切板')
        return data
      } catch (err) {
        ElMessage.error('保存失败')
        return null
      }
    },

    async togglePin(record) {
      try {
        await clipboardAPI.togglePin(record.id, !record.pinned)
        record.pinned = !record.pinned
      } catch (err) {
        ElMessage.error('操作失败')
      }
    },

    async deleteRecord(record) {
      try {
        await clipboardAPI.delete(record.id)
        this.records = this.records.filter(r => r.id !== record.id)
        ElMessage.success('已删除')
      } catch (err) {
        ElMessage.error('删除失败')
      }
    },

    async clearAll(onlyUnpinned = true) {
      try {
        await clipboardAPI.clear(onlyUnpinned)
        if (onlyUnpinned) {
          this.records = this.records.filter(r => r.pinned)
        } else {
          this.records = []
        }
        ElMessage.success('已清空')
      } catch (err) {
        ElMessage.error('清空失败')
      }
    },

    startPolling(intervalMs = 5000) {
      this.stopPolling()
      this.pollingInterval = setInterval(() => {
        this.fetchRecords()
      }, intervalMs)
    },

    stopPolling() {
      if (this.pollingInterval) {
        clearInterval(this.pollingInterval)
        this.pollingInterval = null
      }
    }
  }
})