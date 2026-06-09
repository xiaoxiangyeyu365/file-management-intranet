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
      const tempMsg = {
        id: Date.now() + 1,
        role: 'assistant',
        content: '',
        sources: null,
        status: 'streaming',
        createdAt: new Date().toISOString()
      }
      this.messages.push(tempMsg)

      this.isStreaming = true
      this.streamError = null

      try {
        const resp = await chatAPI.ask(this.currentConversation.id, question)

        if (!resp.ok) {
          const errData = await resp.json()
          throw new Error(errData.message || 'Request failed')
        }

        const reader = resp.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split('\n')
          buffer = lines.pop() || ''

          for (const line of lines) {
            const trimmed = line.trim()
            if (!trimmed) continue

            if (trimmed.startsWith('data: ')) {
              const data = trimmed.slice(6)
              try {
                const parsed = JSON.parse(data)

                if (parsed.content !== undefined) {
                  tempMsg.content += parsed.content
                }
                if (parsed.messageId !== undefined) {
                  tempMsg.id = parsed.messageId
                  tempMsg.status = 'done'
                  tempMsg.sources = parsed.sources || null
                }
                if (parsed.error) {
                  tempMsg.status = 'error'
                  this.streamError = parsed.error
                }
              } catch (e) {
                // Skip unparseable data lines
              }
            }
          }
        }

        // Ensure final state
        if (tempMsg.status === 'streaming') {
          tempMsg.status = 'done'
        }
      } catch (e) {
        tempMsg.status = 'error'
        this.streamError = e.message
      } finally {
        this.isStreaming = false
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
