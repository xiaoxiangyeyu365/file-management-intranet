import { defineStore } from 'pinia'
import { authAPI } from '@/utils/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null,
    token: localStorage.getItem('cloudbox_token') || null,
    requirePasswordChange: false
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
      this.requirePasswordChange = data.requirePasswordChange || false
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
      this.requirePasswordChange = false
      localStorage.removeItem('cloudbox_token')
    },

    async changePassword(oldPassword, newPassword) {
      const result = await authAPI.changePassword(oldPassword, newPassword)
      this.requirePasswordChange = false
      if (this.user) {
        this.user.passwordChanged = true
      }
      return result
    },

    async fetchProfile() {
      if (!this.token) return null
      try {
        const response = await authAPI.profile()
        const userData = response.data || response
        this.user = {
          id: userData.id,
          username: userData.username,
          role: userData.role,
          passwordChanged: userData.passwordChanged
        }
        if (!userData.passwordChanged) {
          this.requirePasswordChange = true
        }
        return this.user
      } catch (error) {
        this.logout()
        return null
      }
    }
  }
})
