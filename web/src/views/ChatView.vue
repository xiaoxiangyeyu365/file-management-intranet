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
        <div class="chat-files-bar">
          <div class="conversation-files">
            <el-tag
              v-for="f in conversationFiles"
              :key="f.id"
              size="small"
              type="info"
              class="conversation-file-tag"
              closable
              @close="handleRemoveFile(f)"
            >
              {{ f.name }}
            </el-tag>
            <span v-if="conversationFiles.length === 0" class="no-files">本对话还没有关联文件</span>
          </div>
          <el-button text size="small" :icon="FolderAdd" @click="openAddFileDialog">
            添加文件
          </el-button>
        </div>

        <!-- Messages -->
        <div class="message-area" ref="messageArea">
          <div
            v-for="msg in chatStore.messages"
            :key="msg.id"
            class="message"
            :class="[msg.role, msg.status]"
          >
            <div class="message-bubble">
              <div
                v-if="msg.status !== 'error' && msg.role === 'assistant'"
                class="message-content markdown-body"
                v-html="renderMarkdown(msg.content)"
              ></div>
              <div v-else-if="msg.status !== 'error'" class="message-content">{{ msg.content }}</div>
              <div v-if="msg.status === 'error'" class="message-error">
                <el-icon :size="14"><WarningFilled /></el-icon>
                <span>{{ msg.content || '回复中断' }}</span>
                <el-button size="small" text type="primary" @click="chatStore.retryAsk()">
                  重试
                </el-button>
              </div>
            </div>
            <div v-if="msg.role === 'assistant' && msg.status === 'stopped'" class="message-stopped">
              已停止生成
            </div>
            <div
              v-if="msg.role === 'assistant' && msg.content && msg.status !== 'streaming'"
              class="message-actions"
            >
              <el-button text size="small" :icon="CopyDocument" @click="copyMessage(msg)">
                复制
              </el-button>
            </div>
            <!-- Sources -->
            <div v-if="msg.role === 'assistant' && msg.sources?.length" class="message-sources">
              <el-collapse>
                <el-collapse-item title="引用来源">
                  <div v-for="(src, i) in msg.sources" :key="i" class="source-item">
                    <button type="button" class="source-file" @click="openSource(src)">
                      {{ src.fileName || '未命名文件' }}
                    </button>
                    <span v-if="src.score != null" class="source-score">
                      {{ Math.round(src.score * 100) }}%
                    </span>
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
                v-if="chatStore.isStreaming"
                :icon="VideoPause"
                @click="chatStore.stopStreaming"
              >
                停止
              </el-button>
              <el-button v-else :icon="Promotion" @click="handleAsk" />
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
    <el-dialog v-model="fileDialogVisible" :title="fileDialogMode === 'add' ? '添加文件到对话' : '选择文件'" width="90%" :fullscreen="isMobile">
      <el-input
        v-model="fileSearch"
        placeholder="搜索文件..."
        clearable
        class="file-search-input"
      />
      <div class="file-select-list">
        <el-checkbox-group v-model="selectedFileIds">
          <div v-for="file in filteredAvailableFiles" :key="file.id" class="file-select-item">
            <el-checkbox :label="file.id">
              {{ file.name }}
              <span class="file-meta">({{ file.chunkCount }} 个片段)</span>
            </el-checkbox>
          </div>
        </el-checkbox-group>
        <div v-if="filteredAvailableFiles.length === 0" class="empty-hint">
          {{ emptyFileHint }}
        </div>
      </div>
      <template #footer>
        <el-button @click="fileDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmFileSelection" :disabled="selectedFileIds.length === 0">
          {{ fileDialogMode === 'add' ? '添加' : '确定' }} ({{ selectedFileIds.length }} 个文件)
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
import { Delete, Promotion, Menu, EditPen, WarningFilled, FolderAdd, CopyDocument, VideoPause } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'

const md = new MarkdownIt({ html: false, linkify: true, breaks: true })

DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A') {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})

function renderMarkdown(text) {
  return DOMPurify.sanitize(md.render(text || ''))
}

const route = useRoute()
const router = useRouter()
const chatStore = useChatStore()

const inputText = ref('')
const messageArea = ref(null)
const fileDialogVisible = ref(false)
const fileDialogMode = ref('create')
const fileSearch = ref('')
const availableFiles = ref([])
const selectedFileIds = ref([])
const sidebarOpen = ref(false)

const isMobile = ref(window.innerWidth < 768)

const currentId = computed(() => {
  const id = route.params.id
  return id ? parseInt(id) : null
})

const filteredAvailableFiles = computed(() => {
  const keyword = fileSearch.value.trim().toLowerCase()
  return availableFiles.value.filter(file => {
    if (fileDialogMode.value === 'add' && chatStore.currentFileIds.includes(file.id)) {
      return false
    }
    return !keyword || file.name.toLowerCase().includes(keyword)
  })
})

const conversationFiles = computed(() => {
  const byId = new Map(availableFiles.value.map(file => [file.id, file]))
  return chatStore.currentFileIds.map(id => byId.get(id)).filter(Boolean)
})

const emptyFileHint = computed(() => {
  if (availableFiles.value.length === 0) return '暂无可用的已索引文件，请先上传文档'
  if (fileSearch.value.trim()) return '没有匹配的文件'
  if (fileDialogMode.value === 'add') return '该对话已包含全部可用文件'
  return '没有可选择的文件'
})

function updateIsMobile() {
  isMobile.value = window.innerWidth < 768
}

onMounted(() => {
  window.addEventListener('resize', updateIsMobile)
  chatStore.loadConversations()
  loadAvailableFiles()
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

async function loadAvailableFiles(warn = false) {
  try {
    const res = await chatAPI.availableFiles()
    const data = res.data || res
    if (data && data.code !== undefined && data.code !== 0) {
      if (warn) ElMessage.warning(data.message || '无法加载可用文件')
      availableFiles.value = []
      return
    }
    availableFiles.value = data.files || []
  } catch (e) {
    console.error('Failed to load available files', e)
    if (warn) ElMessage.error(e.response?.data?.message || '无法加载可用文件')
  }
}

async function openFileDialog(mode) {
  sidebarOpen.value = false
  fileDialogMode.value = mode
  fileSearch.value = ''
  selectedFileIds.value = []
  await loadAvailableFiles(true)
  fileDialogVisible.value = true
}

function handleNewConversation() {
  return openFileDialog('create')
}

function openAddFileDialog() {
  if (!chatStore.currentConversation) return
  return openFileDialog('add')
}

async function confirmFileSelection() {
  if (selectedFileIds.value.length === 0) return
  fileDialogVisible.value = false

  if (fileDialogMode.value === 'add') {
    try {
      await chatStore.addFiles(selectedFileIds.value)
      ElMessage.success('已添加文件')
    } catch (e) {
      ElMessage.error(e.response?.data?.message || e.message || '添加文件失败')
    }
    return
  }

  try {
    const conv = await chatStore.createConversation(selectedFileIds.value)
    router.push(`/chat/${conv.id}`)
  } catch (e) {
    ElMessage.error(e.response?.data?.message || e.message || '创建对话失败')
  }
}

async function handleRemoveFile(file) {
  try {
    await ElMessageBox.confirm(`从对话中移除「${file.name}」？`, '移除文件', {
      confirmButtonText: '移除',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }

  try {
    await chatStore.removeFile(file.id)
    ElMessage.success('已移除')
  } catch (e) {
    ElMessage.error(e.response?.data?.message || e.message || '移除失败')
  }
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

function openSource(src) {
  if (!src?.fileId) return
  router.push({ path: '/', query: { preview: String(src.fileId) } })
}

async function copyMessage(msg) {
  const text = msg.content || ''
  if (!text) return
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      const ta = document.createElement('textarea')
      ta.value = text
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    ElMessage.success('已复制')
  } catch (e) {
    ElMessage.error('复制失败')
  }
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

.chat-files-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid #f0f2f5;
  background: #fff;
  flex-shrink: 0;

  .conversation-files {
    display: flex;
    align-items: center;
    gap: 6px;
    overflow-x: auto;
    white-space: nowrap;
    min-width: 0;
  }

  .no-files {
    font-size: 12px;
    color: #909399;
  }
}

.file-search-input {
  margin-bottom: 10px;
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

.message-content.markdown-body {
  white-space: normal;
}

.markdown-body {
  :deep(p) {
    margin: 0 0 8px;
  }

  :deep(p:last-child) {
    margin-bottom: 0;
  }

  :deep(h1),
  :deep(h2),
  :deep(h3),
  :deep(h4) {
    margin: 12px 0 6px;
    font-weight: 600;
    line-height: 1.4;
  }

  :deep(h1) { font-size: 18px; }
  :deep(h2) { font-size: 16px; }
  :deep(h3), :deep(h4) { font-size: 15px; }

  :deep(ul),
  :deep(ol) {
    margin: 6px 0;
    padding-left: 22px;
  }

  :deep(li) {
    margin: 2px 0;
  }

  :deep(code) {
    padding: 1px 4px;
    border-radius: 3px;
    background: #e9ecef;
    font-family: Consolas, Monaco, 'Courier New', monospace;
    font-size: 12.5px;
  }

  :deep(pre) {
    margin: 8px 0;
    padding: 10px 12px;
    border-radius: 6px;
    background: #1e1e1e;
    color: #d4d4d4;
    overflow-x: auto;
  }

  :deep(pre code) {
    padding: 0;
    background: transparent;
    color: inherit;
    font-size: 12.5px;
  }

  :deep(blockquote) {
    margin: 8px 0;
    padding-left: 10px;
    border-left: 3px solid #dcdfe6;
    color: #606266;
  }

  :deep(table) {
    margin: 8px 0;
    border-collapse: collapse;
    width: 100%;
  }

  :deep(th),
  :deep(td) {
    border: 1px solid #dcdfe6;
    padding: 4px 8px;
    text-align: left;
  }

  :deep(th) {
    background: #f5f7fa;
  }

  :deep(a) {
    color: #409eff;
  }

  :deep(hr) {
    margin: 10px 0;
    border: none;
    border-top: 1px solid #dcdfe6;
  }
}

.message-actions {
  display: flex;
  gap: 4px;
  margin-top: 4px;
  max-width: 80%;
}

.message-stopped {
  margin-top: 4px;
  max-width: 80%;
  font-size: 12px;
  color: #909399;
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
    padding: 0;
    border: none;
    background: none;
    color: #409eff;
    margin-right: 8px;
    font: inherit;
    font-weight: 500;
    cursor: pointer;

    &:hover {
      text-decoration: underline;
    }
  }

  .source-score {
    margin-right: 8px;
    color: #67c23a;
    font-variant-numeric: tabular-nums;
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
