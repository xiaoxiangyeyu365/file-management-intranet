import { defineStore } from 'pinia'
import { authAPI } from '@/utils/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null,
    token: localStorage.getItem('cloudbox_token') || null
  }),

  getters: {
    isLoggedIn: (state) => !!state.token,
    isAdmin: (state) => state.user?.role === 'admin'
  },

  actions: {
    async login(username, password) {
      const response = await authAPI.login(username, password)
      const data = response.data || response
      this.token = data.token
      localStorage.setItem('cloudbox_token', data.token)
      await this.fetchProfile()
      return data
    },

    async register(username, password) {
      const response = await authAPI.register(username, password)
      return response.data || response
    },

    logout() {
      this.token = null
      this.user = null
      localStorage.removeItem('cloudbox_token')
    },

    async changePassword(oldPassword, newPassword) {
      return await authAPI.changePassword(oldPassword, newPassword)
    },

    async fetchProfile() {
      if (!this.token) return null
      try {
        const response = await authAPI.profile()
        const userData = response.data || response
        this.user = {
          id: userData.id,
          username: userData.username,
          role: userData.role
        }
        return this.user
      } catch (error) {
        this.logout()
        return null
      }
    }
  }
})
