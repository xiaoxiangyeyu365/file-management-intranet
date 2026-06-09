<template>
  <div class="chat-container">
    <!-- Conversation List -->
    <div class="chat-sidebar">
      <div class="chat-sidebar-header">
        <h3>AI 问答</h3>
        <el-button type="primary" size="small" @click="handleNewConversation">
          新对话
        </el-button>
      </div>
      <div class="conversation-list">
        <div
          v-for="conv in chatStore.conversations"
          :key="conv.id"
          class="conversation-item"
          :class="{ active: currentId === conv.id }"
          @click="selectConversation(conv.id)"
        >
          <span class="conv-title">{{ conv.title || '新对话' }}</span>
          <el-button
            class="delete-btn"
            type="danger"
            :icon="Delete"
            size="small"
            text
            @click.stop="handleDelete(conv.id)"
          />
        </div>
        <div v-if="chatStore.conversations.length === 0" class="empty-hint">
          暂无对话，点击上方按钮创建
        </div>
      </div>
    </div>

    <!-- Chat Area -->
    <div class="chat-main">
      <template v-if="chatStore.currentConversation">
        <!-- Messages -->
        <div class="message-area" ref="messageArea">
          <div
            v-for="msg in chatStore.messages"
            :key="msg.id"
            class="message"
            :class="[msg.role, msg.status]"
          >
            <div class="message-bubble">
              <div class="message-content">{{ msg.content }}</div>
              <div v-if="msg.status === 'error'" class="message-error">
                回复中断
                <el-button size="small" text type="primary" @click="chatStore.retryAsk()">
                  重试
                </el-button>
              </div>
            </div>
            <!-- Sources -->
            <div v-if="msg.role === 'assistant' && msg.sources?.length" class="message-sources">
              <el-collapse>
                <el-collapse-item title="引用来源">
                  <div v-for="(src, i) in msg.sources" :key="i" class="source-item">
                    <span class="source-file">{{ src.fileName }}</span>
                    <span class="source-preview">{{ src.preview }}</span>
                  </div>
                </el-collapse-item>
              </el-collapse>
            </div>
          </div>
          <div v-if="chatStore.isLoading" class="loading-hint">加载中...</div>
        </div>

        <!-- Input -->
        <div class="input-area">
          <el-input
            v-model="inputText"
            placeholder="输入问题..."
            :disabled="chatStore.isStreaming"
            @keyup.enter="handleAsk"
          >
            <template #append>
              <el-button
                :icon="Promotion"
                :loading="chatStore.isStreaming"
                @click="handleAsk"
              />
            </template>
          </el-input>
        </div>
      </template>

      <template v-else>
        <div class="empty-state">
          <p>选择一个对话或创建新对话开始</p>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useChatStore } from '@/stores/chat'
import { Delete, Promotion } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus'
import { computed } from 'vue'

const route = useRoute()
const router = useRouter()
const chatStore = useChatStore()

const inputText = ref('')
const messageArea = ref(null)

const currentId = computed(() => {
  const id = route.params.id
  return id ? parseInt(id) : null
})

onMounted(() => {
  chatStore.loadConversations()
  if (currentId.value) {
    chatStore.loadConversation(currentId.value)
  }
})

watch(currentId, (id) => {
  if (id) {
    chatStore.loadConversation(id)
  } else {
    chatStore.currentConversation = null
    chatStore.messages = []
  }
})

// Auto-scroll on new messages
watch(() => chatStore.messages.length, () => {
  scrollToBottom()
})

// Auto-scroll during streaming
watch(() => {
  const last = chatStore.messages[chatStore.messages.length - 1]
  return last?.content?.length || 0
}, () => {
  scrollToBottom()
})

function scrollToBottom() {
  nextTick(() => {
    if (messageArea.value) {
      messageArea.value.scrollTop = messageArea.value.scrollHeight
    }
  })
}

async function handleNewConversation() {
  const conv = await chatStore.createConversation()
  router.push(`/chat/${conv.id}`)
}

function selectConversation(id) {
  router.push(`/chat/${id}`)
}

async function handleDelete(id) {
  try {
    await ElMessageBox.confirm('确定删除此对话？', '确认删除', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await chatStore.deleteConversation(id)
    if (currentId.value === id) {
      router.push('/chat')
    }
  } catch {
    // User cancelled
  }
}

async function handleAsk() {
  const q = inputText.value.trim()
  if (!q || chatStore.isStreaming) return
  inputText.value = ''
  await chatStore.ask(q)
}
</script>

<style scoped lang="scss">
.chat-container {
  display: flex;
  height: calc(100vh - 56px);
  background: #fff;
}

.chat-sidebar {
  width: 260px;
  border-right: 1px solid #e4e7ed;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

.chat-sidebar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  border-bottom: 1px solid #e4e7ed;

  h3 {
    margin: 0;
    font-size: 16px;
  }
}

.conversation-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.conversation-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px;
  border-radius: 8px;
  cursor: pointer;
  margin-bottom: 4px;

  &:hover {
    background: #f5f7fa;
  }

  &.active {
    background: #ecf5ff;
    color: #409eff;
  }

  .conv-title {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 14px;
  }

  .delete-btn {
    opacity: 0;
    transition: opacity 0.2s;
  }

  &:hover .delete-btn {
    opacity: 1;
  }
}

.empty-hint {
  text-align: center;
  color: #909399;
  padding: 32px 16px;
  font-size: 14px;
}

.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.message-area {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.message {
  margin-bottom: 16px;
  display: flex;
  flex-direction: column;

  &.user {
    align-items: flex-end;

    .message-bubble {
      background: #ecf5ff;
      color: #303133;
    }
  }

  &.assistant {
    align-items: flex-start;

    .message-bubble {
      background: #f4f4f5;
      color: #303133;
    }
  }

  &.error .message-bubble {
    background: #fef0f0;
  }
}

.message-bubble {
  max-width: 80%;
  padding: 12px 16px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.message-error {
  margin-top: 8px;
  color: #f56c6c;
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.message-sources {
  margin-top: 8px;
  max-width: 80%;

  :deep(.el-collapse-item__header) {
    font-size: 12px;
    height: 28px;
  }
}

.source-item {
  padding: 4px 0;
  font-size: 12px;

  .source-file {
    color: #409eff;
    margin-right: 8px;
    font-weight: 500;
  }

  .source-preview {
    color: #606266;
  }
}

.input-area {
  padding: 16px;
  border-top: 1px solid #e4e7ed;
}

.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #909399;
  font-size: 16px;
}

.loading-hint {
  text-align: center;
  color: #909399;
  padding: 16px;
}

@media (max-width: 767px) {
  .chat-sidebar {
    display: none;
  }
}
</style>
