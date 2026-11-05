import { defineStore } from 'pinia'
import { chatAPI } from '@/utils/api'

export const useChatStore = defineStore('chat', {
  state: () => ({
    conversations: [],
    total: 0,
    currentConversation: null,
    messages: [],
    isLoading: false,
    isStreaming: false,
    streamError: null
  }),

  actions: {
    async loadConversations(limit = 20, offset = 0) {
      try {
        const res = await chatAPI.listConversations(limit, offset)
        const data = res.data || res
        this.conversations = data.conversations || []
        this.total = data.total || 0
      } catch (e) {
        console.error('Failed to load conversations', e)
      }
    },

    async createConversation(fileIds = []) {
      const res = await chatAPI.createConversation({ fileIds })
      const conv = res.data || res
      this.conversations.unshift(conv)
      this.total++
      return conv
    },

    async loadConversation(id) {
      this.isLoading = true
      try {
        const res = await chatAPI.getConversation(id)
        const data = res.data || res
        this.currentConversation = data.conversation
        this.messages = data.messages || []
      } catch (e) {
        console.error('Failed to load conversation', e)
        this.currentConversation = null
        this.messages = []
      } finally {
        this.isLoading = false
      }
    },

    async deleteConversation(id) {
      await chatAPI.deleteConversation(id)
      this.conversations = this.conversations.filter(c => c.id !== id)
      this.total--
      if (this.currentConversation?.id === id) {
        this.currentConversation = null
        this.messages = []
      }
    },

    async addFile(fileId) {
      if (!this.currentConversation) return
      await chatAPI.addFile(this.currentConversation.id, fileId)
    },

    async ask(question) {
      if (!this.currentConversation || this.isStreaming) return

      // Add user message locally
      this.messages.push({
        id: Date.now(),
        role: 'user',
        content: question,
        createdAt: new Date().toISOString()
      })

      // Add temporary assistant message
      const msgIdx = this.messages.length
      this.messages.push({
        id: Date.now() + 1,
        role: 'assistant',
        content: '',
        sources: null,
        status: 'streaming',
        createdAt: new Date().toISOString()
      })

      this.isStreaming = true
      this.streamError = null

      chatAPI.ask(
        this.currentConversation.id,
        question,
        // onToken — access via store index to maintain reactivity
        (content) => {
          this.messages[msgIdx].content += content
        },
        // onDone
        (data) => {
          const msg = this.messages[msgIdx]
          if (!msg) return
          msg.id = data.messageId || msg.id
          msg.status = 'done'
          msg.sources = data.sources || null
          this.isStreaming = false
        },
        // onError
        (error) => {
          const msg = this.messages[msgIdx]
          if (!msg) return
          msg.status = 'error'
          msg.content = error
          this.streamError = error
          this.isStreaming = false
        }
      )
    },

    async retryAsk() {
      // Find last user message
      const lastUserMsg = [...this.messages].reverse().find(m => m.role === 'user')
      if (!lastUserMsg) return

      // Remove failed assistant message
      const lastMsg = this.messages[this.messages.length - 1]
      if (lastMsg?.role === 'assistant' && lastMsg.status === 'error') {
        this.messages.pop()
      }

      await this.ask(lastUserMsg.content)
    }
  }
})
