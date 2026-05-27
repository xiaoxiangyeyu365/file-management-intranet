import { defineStore } from 'pinia'
import { adminAPI } from '@/utils/api'

export const useAdminStore = defineStore('admin', {
  state: () => ({
    users: [],
    total: 0,
    pendingCount: 0,
    loading: false,
    statusFilter: ''
  }),

  actions: {
    async fetchUsers(status = '') {
      this.loading = true
      try {
        const response = await adminAPI.listUsers(status)
        const data = response.data || response
        this.users = data.users || []
        this.total = data.total || 0
        this.pendingCount = data.pendingCount || 0
        this.statusFilter = status
      } finally {
        this.loading = false
      }
    },

    async createUser(username, password, role) {
      await adminAPI.createUser(username, password, role)
      await this.fetchUsers(this.statusFilter)
    },

    async updateUser(id, data) {
      await adminAPI.updateUser(id, data)
      await this.fetchUsers(this.statusFilter)
    },

    async resetPassword(id, newPassword) {
      await adminAPI.resetPassword(id, newPassword)
    },

    async deleteUser(id) {
      await adminAPI.deleteUser(id)
      await this.fetchUsers(this.statusFilter)
    }
  }
})
