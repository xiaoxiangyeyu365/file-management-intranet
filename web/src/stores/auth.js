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
      // Interceptor returns {code, message, data}; actual payload in .data
      const data = response.data || response
      this.token = data.token
      this.user = { id: data.userId || 1, username: username }
      localStorage.setItem('cloudbox_token', data.token)
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
      if (!this.token) {
        console.log('fetchProfile: no token')
        return null
      }
      try {
        console.log('fetchProfile: calling API')
        const response = await authAPI.profile()
        console.log('fetchProfile response:', response)
        const userData = response.data || response
        this.user = { id: userData.id || 1, username: userData.username }
        return this.user
      } catch (error) {
        console.error('fetchProfile error:', error)
        this.logout()
        return null
      }
    }
  }
})
