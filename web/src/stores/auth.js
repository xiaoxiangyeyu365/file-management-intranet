import { defineStore } from 'pinia'
import { authAPI } from '@/utils/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null,
    token: localStorage.getItem('cloudbox_token') || null
  }),

  getters: {
    isLoggedIn: (state) => !!state.token
  },

  actions: {
    async login(username, password) {
      const response = await authAPI.login(username, password)
      this.token = response.token
      this.user = response.user
      localStorage.setItem('cloudbox_token', response.token)
      return response
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
        const user = await authAPI.profile()
        this.user = user
        return user
      } catch (error) {
        this.logout()
        return null
      }
    }
  }
})