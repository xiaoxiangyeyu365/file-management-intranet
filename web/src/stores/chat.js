import { defineStore } from 'pinia'
import { chatAPI } from '@/utils/api'

export const useChatStore = defineStore('chat', {
  state: () => ({
    conversations: [],
    total: 0,
    currentConversation: null,
    currentFileIds: [],
    messages: [],
    isLoading: false,
    isStreaming: false,
    streamController: null,
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
        this.currentFileIds = data.fileIds || []
        this.messages = data.messages || []
      } catch (e) {
        console.error('Failed to load conversation', e)
        this.currentConversation = null
        this.currentFileIds = []
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
        this.currentFileIds = []
        this.messages = []
      }
    },

    async addFile(fileId) {
      if (!this.currentConversation) return
      await chatAPI.addFile(this.currentConversation.id, fileId)
    },

    // Add files one by one (the backend rewrites file ids as JSON, so
    // parallel requests could overwrite each other) and refresh the
    // conversation so the file list is accurate.
    async addFiles(fileIds) {
      if (!this.currentConversation || !fileIds.length) return
      for (const id of fileIds) {
        await chatAPI.addFile(this.currentConversation.id, id)
      }
      await this.loadConversation(this.currentConversation.id)
    },

    async removeFile(fileId) {
      if (!this.currentConversation) return
      await chatAPI.removeFile(this.currentConversation.id, fileId)
      await this.loadConversation(this.currentConversation.id)
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
      this.streamController = new AbortController()

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
          this.streamController = null
        },
        // onError
        (error) => {
          const msg = this.messages[msgIdx]
          if (!msg) return
          msg.status = 'error'
          msg.content = error
          this.streamError = error
          this.isStreaming = false
          this.streamController = null
        },
        this.streamController.signal
      )
    },

    stopStreaming() {
      if (this.streamController) {
        this.streamController.abort()
        this.streamController = null
      }
      this.isStreaming = false
      const last = this.messages[this.messages.length - 1]
      if (last?.role === 'assistant' && last.status === 'streaming') {
        last.status = 'stopped'
      }
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
