<template>
  <div class="chat-container">
    <!-- Mobile Header Bar -->
    <div class="mobile-header">
      <el-button text @click="sidebarOpen = true" class="menu-btn">
        <el-icon :size="20"><Menu /></el-icon>
      </el-button>
      <span class="mobile-title">{{ chatStore.currentConversation?.title || 'AI 问答' }}</span>
      <el-button text @click="handleNewConversation" class="new-btn">
        <el-icon :size="20"><EditPen /></el-icon>
      </el-button>
    </div>

    <!-- Sidebar Overlay -->
    <div v-if="sidebarOpen" class="sidebar-overlay" @click="sidebarOpen = false" />

    <!-- Conversation List Sidebar -->
    <div class="chat-sidebar" :class="{ open: sidebarOpen }">
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
              <div v-if="msg.status !== 'error'" class="message-content">{{ msg.content }}</div>
              <div v-if="msg.status === 'error'" class="message-error">
                <el-icon :size="14"><WarningFilled /></el-icon>
                <span>{{ msg.content || '回复中断' }}</span>
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

    <!-- File Selection Dialog -->
    <el-dialog v-model="fileDialogVisible" title="选择文件" width="90%" :fullscreen="isMobile">
      <div class="file-select-list">
        <el-checkbox-group v-model="selectedFileIds">
          <div v-for="file in availableFiles" :key="file.id" class="file-select-item">
            <el-checkbox :label="file.id">
              {{ file.name }}
              <span class="file-meta">({{ file.chunkCount }} 个片段)</span>
            </el-checkbox>
          </div>
        </el-checkbox-group>
        <div v-if="availableFiles.length === 0" class="empty-hint">暂无可用的已索引文件，请先上传文档</div>
      </div>
      <template #footer>
        <el-button @click="fileDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmFileSelection" :disabled="selectedFileIds.length === 0">
          确定 ({{ selectedFileIds.length }} 个文件)
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, onMounted, onUnmounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useChatStore } from '@/stores/chat'
import { chatAPI } from '@/utils/api'
import { Delete, Promotion, Menu, EditPen, WarningFilled } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus'

const route = useRoute()
const router = useRouter()
const chatStore = useChatStore()

const inputText = ref('')
const messageArea = ref(null)
const fileDialogVisible = ref(false)
const availableFiles = ref([])
const selectedFileIds = ref([])
const sidebarOpen = ref(false)

const isMobile = ref(window.innerWidth < 768)

const currentId = computed(() => {
  const id = route.params.id
  return id ? parseInt(id) : null
})

function updateIsMobile() {
  isMobile.value = window.innerWidth < 768
}

onMounted(() => {
  window.addEventListener('resize', updateIsMobile)
  chatStore.loadConversations()
  if (currentId.value) {
    chatStore.loadConversation(currentId.value)
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', updateIsMobile)
})

watch(currentId, (id) => {
  sidebarOpen.value = false
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

// Auto-scroll during streaming (only if user is already near the bottom)
watch(() => {
  const last = chatStore.messages[chatStore.messages.length - 1]
  return last?.content?.length || 0
}, () => {
  if (isNearBottom()) {
    scrollToBottom()
  }
})

function isNearBottom(threshold = 60) {
  if (!messageArea.value) return true
  const el = messageArea.value
  return el.scrollHeight - el.scrollTop - el.clientHeight < threshold
}

function scrollToBottom() {
  nextTick(() => {
    if (messageArea.value) {
      messageArea.value.scrollTop = messageArea.value.scrollHeight
    }
  })
}

async function handleNewConversation() {
  sidebarOpen.value = false
  try {
    const res = await chatAPI.availableFiles()
    const data = res.data || res
    availableFiles.value = data.files || []
    selectedFileIds.value = []
    fileDialogVisible.value = true
  } catch (e) {
    console.error('Failed to load available files', e)
  }
}

async function confirmFileSelection() {
  fileDialogVisible.value = false
  const conv = await chatStore.createConversation(selectedFileIds.value)
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

async function handleAsk(e) {
  // Shift+Enter: allow newlines, Enter: send
  if (e && e.shiftKey) return
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
  position: relative;
  overflow: hidden;
}

// Mobile Header (hidden on desktop)
.mobile-header {
  display: none;
}

// Sidebar Overlay (hidden on desktop)
.sidebar-overlay {
  display: none;
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
  -webkit-overflow-scrolling: touch;
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

.file-select-list {
  max-height: 400px;
  overflow-y: auto;
}

.file-select-item {
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;

  .file-meta {
    color: #909399;
    font-size: 12px;
    margin-left: 4px;
  }
}

// ---- Mobile Styles ----
@media (max-width: 767px) {
  .chat-container {
    flex-direction: column;
    height: 100dvh; // dynamic viewport height for mobile keyboards
  }

  .mobile-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-bottom: 1px solid #e4e7ed;
    background: #fff;
    z-index: 10;
    flex-shrink: 0;

    .mobile-title {
      flex: 1;
      text-align: center;
      font-size: 16px;
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .menu-btn, .new-btn {
      width: 40px;
      height: 40px;
      padding: 0;
    }
  }

  .sidebar-overlay {
    display: block;
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.3);
    z-index: 100;
  }

  .chat-sidebar {
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    width: 280px;
    z-index: 101;
    background: #fff;
    transform: translateX(-100%);
    transition: transform 0.25s ease;
    box-shadow: none;

    &.open {
      transform: translateX(0);
      box-shadow: 2px 0 12px rgba(0, 0, 0, 0.15);
    }
  }

  .conversation-item {
    padding: 14px 12px;

    // Always show delete button on mobile (no hover on touch)
    .delete-btn {
      opacity: 0.6;
    }
  }

  .chat-main {
    flex: 1;
    min-height: 0; // allow flex child to shrink for keyboard
  }

  .message-area {
    padding: 12px;
    padding-bottom: 8px;
  }

  .message-bubble {
    max-width: 88%; // slightly wider on small screens
    padding: 10px 14px;
    font-size: 15px;
    border-radius: 16px;
  }

  .message-sources {
    max-width: 88%;
  }

  .input-area {
    padding: 8px 12px;
    padding-bottom: max(8px, env(safe-area-inset-bottom)); // safe area for notch devices
    border-top: 1px solid #e4e7ed;
    background: #fff;
    flex-shrink: 0;

    :deep(.el-input__wrapper) {
      border-radius: 20px;
    }
  }

  .empty-state {
    padding: 0 24px;
    text-align: center;

    p {
      font-size: 14px;
    }
  }

  .file-select-list {
    max-height: 60vh;
  }
}
</style>
