import { defineStore } from 'pinia'
import { chatAPI } from '@/utils/api'

// Kept outside Pinia state: AbortController should not be wrapped by Vue's
// reactivity system.
let streamController = null

// Sources are stored as a JSON string in the messages table.
function parseSources(raw) {
  if (!raw) return null
  if (Array.isArray(raw)) return raw
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : null
  } catch {
    return null
  }
}

export const useChatStore = defineStore('chat', {
  state: () => ({
    conversations: [],
    total: 0,
    currentConversation: null,
    currentFileIds: [],
    messages: [],
    isLoading: false,
    isStreaming: false,
    streamError: null
  }),

  actions: {
    async loadConversations(limit = 100, offset = 0) {
      try {
        const res = await chatAPI.listConversations(limit, offset)
        const data = res.data || res
        this.conversations = data.conversations || []
        this.total = data.total || 0
      } catch (e) {
        console.error('Failed to load conversations', e)
      }
    },

    async createConversation(fileIds = [], scope = 'files') {
      const res = await chatAPI.createConversation({ fileIds, scope })
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
        this.messages = (data.messages || []).map(msg => ({
          ...msg,
          sources: parseSources(msg.sources)
        }))
      } catch (e) {
        console.error('Failed to load conversation', e)
        this.currentConversation = null
        this.currentFileIds = []
        this.messages = []
      } finally {
        this.isLoading = false
      }
    },

    async renameConversation(id, title) {
      const res = await chatAPI.renameConversation(id, title)
      const conv = res.data || res
      const item = this.conversations.find(c => c.id === id)
      if (item) item.title = conv.title || title
      if (this.currentConversation?.id === id) {
        this.currentConversation.title = conv.title || title
      }
      return conv
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

      this.messages.push({
        id: Date.now(),
        role: 'user',
        content: question,
        createdAt: new Date().toISOString()
      })

      const msgIdx = this.messages.length
      this.messages.push({
        id: Date.now() + 1,
        role: 'assistant',
        content: '',
        sources: null,
        status: 'streaming',
        createdAt: new Date().toISOString()
      })

      await this._streamAnswer(question, msgIdx)
    },

    async regenerateLast() {
      if (!this.currentConversation || this.isStreaming) return

      let idx = -1
      for (let i = this.messages.length - 1; i >= 0; i--) {
        if (this.messages[i].role === 'assistant') {
          idx = i
          break
        }
      }
      if (idx === -1) return

      const previous = this.messages[idx]
      let question = ''
      for (let i = idx - 1; i >= 0; i--) {
        if (this.messages[i].role === 'user') {
          question = this.messages[i].content
          break
        }
      }
      if (!question) return

      const persistedId = typeof previous.id === 'number' && previous.id < 1e12 ? previous.id : null
      this.messages[idx] = {
        id: Date.now(),
        role: 'assistant',
        content: '',
        sources: null,
        status: 'streaming',
        createdAt: new Date().toISOString()
      }

      await this._streamAnswer(question, idx, { regenerate: true, messageId: persistedId })
    },

    async _streamAnswer(question, msgIdx, options = {}) {
      this.isStreaming = true
      this.streamError = null
      streamController = new AbortController()

      chatAPI.ask(
        this.currentConversation.id,
        question,
        (content) => {
          const msg = this.messages[msgIdx]
          if (msg) msg.content += content
        },
        (data) => {
          const msg = this.messages[msgIdx]
          if (!msg) return
          msg.id = data.messageId || msg.id
          msg.status = 'done'
          msg.sources = data.sources || null
          this.isStreaming = false
          streamController = null
        },
        (error) => {
          const msg = this.messages[msgIdx]
          if (!msg) return
          msg.status = 'error'
          msg.content = error
          this.streamError = error
          this.isStreaming = false
          streamController = null
        },
        streamController.signal,
        options
      )
    },

    stopStreaming() {
      if (streamController) {
        streamController.abort()
        streamController = null
      }
      this.isStreaming = false
      const last = this.messages[this.messages.length - 1]
      if (last?.role === 'assistant' && last.status === 'streaming') {
        last.status = 'stopped'
      }
    },

    async retryAsk() {
      const lastIdx = this.messages.length - 1
      const last = this.messages[lastIdx]
      if (!last || last.role !== 'assistant' || last.status !== 'error') return

      let question = ''
      for (let i = lastIdx - 1; i >= 0; i--) {
        if (this.messages[i].role === 'user') {
          question = this.messages[i].content
          break
        }
      }
      if (!question) return

      const persistedId = typeof last.id === 'number' && last.id < 1e12 ? last.id : null
      this.messages[lastIdx] = {
        id: Date.now(),
        role: 'assistant',
        content: '',
        sources: null,
        status: 'streaming',
        createdAt: new Date().toISOString()
      }

      await this._streamAnswer(question, lastIdx, { regenerate: true, messageId: persistedId })
    }
  }
})
