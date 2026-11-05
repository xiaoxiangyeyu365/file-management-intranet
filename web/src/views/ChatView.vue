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
      <div class="conversation-search">
        <el-input
          v-model="conversationSearch"
          placeholder="搜索对话..."
          size="small"
          clearable
        />
      </div>
      <div class="conversation-list">
        <div
          v-for="conv in filteredConversations"
          :key="conv.id"
          class="conversation-item"
          :class="{ active: currentId === conv.id }"
          @click="selectConversation(conv.id)"
        >
          <span class="conv-title">{{ conv.title || '新对话' }}</span>
          <el-button
            class="rename-btn"
            :icon="EditPen"
            size="small"
            text
            @click.stop="handleRename(conv)"
          />
          <el-button
            class="delete-btn"
            type="danger"
            :icon="Delete"
            size="small"
            text
            @click.stop="handleDelete(conv.id)"
          />
        </div>
        <div v-if="filteredConversations.length === 0" class="empty-hint">
          {{ chatStore.conversations.length === 0 ? '暂无对话，点击上方按钮创建' : '没有匹配的对话' }}
        </div>
      </div>
    </div>

    <!-- Chat Area -->
    <div class="chat-main">
      <template v-if="chatStore.currentConversation">
        <div v-if="!isMobile" class="chat-header">
          <div class="chat-title" :title="chatStore.currentConversation.title">
            {{ chatStore.currentConversation.title || '新对话' }}
          </div>
          <el-button text size="small" :icon="EditPen" @click="handleRename(chatStore.currentConversation)">
            重命名
          </el-button>
        </div>

        <div class="chat-files-bar">
          <div v-if="isAllScope" class="conversation-files">
            <el-tag size="small" type="success">全部知识库</el-tag>
            <span class="no-files">共 {{ availableFiles.length }} 个已索引文件</span>
          </div>
          <div v-else class="conversation-files">
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
          <el-button
            v-if="!isAllScope"
            text
            size="small"
            :icon="FolderAdd"
            @click="openAddFileDialog"
          >
            添加文件
          </el-button>
        </div>

        <!-- Messages -->
        <div class="message-area" ref="messageArea">
          <div v-if="chatStore.messages.length === 0 && !chatStore.isLoading" class="chat-welcome">
            <div class="welcome-icon">✨</div>
            <div class="welcome-title">开始提问</div>
            <div class="welcome-sub">
              {{ isAllScope ? '将基于你全部已索引的文件回答' : '将基于本对话关联的文件回答' }}
            </div>
            <div class="suggestions">
              <button
                v-for="q in suggestedQuestions"
                :key="q"
                type="button"
                class="suggestion"
                :disabled="chatStore.isStreaming"
                @click="askSuggestion(q)"
              >
                {{ q }}
              </button>
            </div>
          </div>

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
                @click="handleCitationClick($event, msg)"
              >
                <span v-if="msg.status === 'streaming' && !msg.content" class="typing-hint">
                  正在检索文档并生成回答…
                </span>
                <span v-else v-html="renderMarkdown(msg.content)"></span>
              </div>
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
              v-if="msg.role === 'assistant' && msg.status !== 'streaming' && (msg.content || isLastAssistant(msg))"
              class="message-actions"
            >
              <el-button
                v-if="msg.content"
                text
                size="small"
                :icon="CopyDocument"
                @click="copyMessage(msg)"
              >
                复制
              </el-button>
              <el-button
                v-if="isLastAssistant(msg)"
                text
                size="small"
                :icon="RefreshRight"
                @click="chatStore.regenerateLast()"
              >
                重新生成
              </el-button>
            </div>
            <div v-if="msg.createdAt" class="message-time">
              {{ formatMessageTime(msg.createdAt) }}
            </div>
            <!-- Sources -->
            <div v-if="msg.role === 'assistant' && msg.sources?.length" class="message-sources">
              <el-collapse>
                <el-collapse-item :title="`引用来源 (${msg.sources.length})`">
                  <div
                    v-for="(src, i) in msg.sources"
                    :key="i"
                    :id="`cite-${i + 1}`"
                    class="source-item"
                  >
                    <span class="source-index">[{{ i + 1 }}]</span>
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
          <div class="input-wrap">
            <el-input
              v-model="inputText"
              type="textarea"
              :autosize="{ minRows: 1, maxRows: 6 }"
              resize="none"
              placeholder="输入问题，Enter 发送，Shift+Enter 换行"
              :disabled="chatStore.isStreaming"
              @keydown="handleInputKeydown"
            />
            <el-button
              v-if="chatStore.isStreaming"
              :icon="VideoPause"
              @click="chatStore.stopStreaming"
            >
              停止
            </el-button>
            <el-button
              v-else
              type="primary"
              :icon="Promotion"
              :disabled="!inputText.trim()"
              @click="handleAsk"
            />
          </div>
        </div>
      </template>

      <template v-else>
        <div class="empty-state">
          <div class="empty-icon">💬</div>
          <p class="empty-title">开始一个新的 AI 对话</p>
          <p class="empty-sub">默认基于你全部已索引的文件回答，也可以指定文件。</p>
          <el-button type="primary" @click="handleNewConversation">新建对话</el-button>
        </div>
      </template>
    </div>

    <!-- File Selection Dialog -->
    <el-dialog v-model="fileDialogVisible" :title="fileDialogMode === 'add' ? '添加文件到对话' : '选择文件范围'" width="90%" :fullscreen="isMobile">
      <div v-if="fileDialogMode === 'create'" class="scope-selector">
        <el-radio-group v-model="fileScope">
          <el-radio-button label="all">全部知识库</el-radio-button>
          <el-radio-button label="files">指定文件</el-radio-button>
        </el-radio-group>
        <div class="scope-hint">
          {{ fileScope === 'all'
            ? `将使用全部已索引文件（当前 ${availableFiles.length} 个），之后新索引的文件也会自动加入。`
            : '选择本对话要使用的文件。' }}
        </div>
      </div>

      <template v-if="fileDialogMode === 'add' || fileScope === 'files'">
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
      </template>
      <template #footer>
        <el-button @click="fileDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmFileSelection" :disabled="!canConfirmFileSelection">
          {{ fileDialogMode === 'add' ? '添加' : '开始对话' }}
          <template v-if="fileDialogMode === 'add' || fileScope === 'files'">({{ selectedFileIds.length }} 个文件)</template>
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
import { Delete, Promotion, Menu, EditPen, WarningFilled, FolderAdd, CopyDocument, VideoPause, RefreshRight } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'

const md = new MarkdownIt({ html: false, linkify: true, breaks: true })

// Render [1], [2] citations as clickable links. Inline rules do not run
// inside code blocks, so array index syntax in code is left alone.
md.inline.ruler.before('link', 'citation', (state, silent) => {
  const start = state.pos
  if (state.src.charCodeAt(start) !== 0x5B) return false
  const match = /^\[(\d+)\]/.exec(state.src.slice(start, state.posMax))
  if (!match) return false
  if (!silent) {
    const token = state.push('citation', '', 0)
    token.content = match[1]
  }
  state.pos = start + match[0].length
  return true
})

md.renderer.rules.citation = (tokens, idx) => {
  const n = tokens[idx].content
  return `<a href="#cite-${n}" class="citation" data-cite="${n}">[${n}]</a>`
}

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
const fileScope = ref('all')
const conversationSearch = ref('')
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

const isAllScope = computed(() => chatStore.currentConversation?.scope === 'all')

const filteredConversations = computed(() => {
  const keyword = conversationSearch.value.trim().toLowerCase()
  if (!keyword) return chatStore.conversations
  return chatStore.conversations.filter(conv =>
    (conv.title || '新对话').toLowerCase().includes(keyword)
  )
})

const suggestionFiles = computed(() => (
  isAllScope.value ? availableFiles.value : conversationFiles.value
))

const suggestedQuestions = computed(() => {
  const list = []
  const names = suggestionFiles.value.slice(0, 2).map(file => file.name)
  if (names[0]) list.push(`总结一下《${names[0]}》的主要内容`)
  if (names[1]) list.push(`《${names[1]}》里有哪些关键结论？`)
  list.push('这些文档主要讲了什么？')
  return list.slice(0, 3)
})

const canConfirmFileSelection = computed(() => {
  if (fileDialogMode.value === 'add') return selectedFileIds.value.length > 0
  if (fileScope.value === 'all') return true
  return selectedFileIds.value.length > 0
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
  fileScope.value = mode === 'add' ? 'files' : 'all'
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
  if (!canConfirmFileSelection.value) return
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

  const scope = fileScope.value
  const fileIds = scope === 'all' ? [] : selectedFileIds.value
  try {
    const conv = await chatStore.createConversation(fileIds, scope)
    conversationSearch.value = ''
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

async function handleAsk() {
  const q = inputText.value.trim()
  if (!q || chatStore.isStreaming) return
  inputText.value = ''
  await chatStore.ask(q)
}

function handleInputKeydown(e) {
  if (e.key !== 'Enter' || e.shiftKey || e.isComposing) return
  e.preventDefault()
  handleAsk()
}

async function askSuggestion(question) {
  if (chatStore.isStreaming) return
  await chatStore.ask(question)
}

function isLastAssistant(msg) {
  return msg.role === 'assistant' && chatStore.messages[chatStore.messages.length - 1] === msg
}

function formatMessageTime(value) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function handleCitationClick(event, msg) {
  const el = event.target.closest?.('.citation')
  if (!el) return
  event.preventDefault()
  const index = Number(el.dataset.cite)
  const source = msg.sources?.[index - 1]
  if (source) openSource(source)
}

async function handleRename(conv) {
  try {
    const { value } = await ElMessageBox.prompt('输入新的对话名称', '重命名对话', {
      inputValue: conv.title || '',
      confirmButtonText: '保存',
      cancelButtonText: '取消',
      inputValidator: (v) => (v && v.trim() ? true : '名称不能为空')
    })
    await chatStore.renameConversation(conv.id, value.trim())
    ElMessage.success('已重命名')
  } catch {
    // Cancelled or failed
  }
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

.conversation-search {
  padding: 8px 12px 0;
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

  .delete-btn,
  .rename-btn {
    opacity: 0;
    transition: opacity 0.2s;
  }

  &:hover .delete-btn,
  &:hover .rename-btn {
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

.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid #f0f2f5;
  flex-shrink: 0;

  .chat-title {
    font-size: 15px;
    font-weight: 600;
    color: #303133;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
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

.scope-selector {
  margin-bottom: 12px;
}

.scope-hint {
  margin-top: 8px;
  color: #909399;
  font-size: 12px;
  line-height: 1.6;
}

.chat-welcome {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 240px;
  padding: 24px;
  text-align: center;

  .welcome-icon {
    font-size: 36px;
  }

  .welcome-title {
    margin-top: 8px;
    font-size: 18px;
    font-weight: 600;
    color: #303133;
  }

  .welcome-sub {
    margin-top: 4px;
    font-size: 13px;
    color: #909399;
  }

  .suggestions {
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: 100%;
    max-width: 520px;
    margin-top: 18px;
  }

  .suggestion {
    padding: 10px 14px;
    border: 1px solid #e4e7ed;
    border-radius: 8px;
    background: #fafafa;
    color: #606266;
    font-size: 13px;
    text-align: left;
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      border-color: #409eff;
      color: #409eff;
      background: #ecf5ff;
    }

    &:disabled {
      cursor: not-allowed;
      opacity: 0.6;
    }
  }
}

.message-area {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  -webkit-overflow-scrolling: touch;
}

.message {
  position: relative;
  margin-bottom: 16px;
  display: flex;
  flex-direction: column;
  padding-left: 44px;

  &::before {
    content: 'AI';
    position: absolute;
    top: 0;
    left: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border-radius: 50%;
    background: #e9e4ff;
    color: #6b4eff;
    font-size: 12px;
    font-weight: 600;
  }

  &.user {
    align-items: flex-end;
    padding-left: 0;
    padding-right: 44px;

    &::before {
      content: '我';
      left: auto;
      right: 0;
      background: #e6f2ff;
      color: #2b7de9;
    }

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

  &.streaming .message-bubble::after {
    content: '▍';
    margin-left: 2px;
    color: #6b4eff;
    animation: blink 1s step-end infinite;
  }

  &.error .message-bubble {
    background: #fef0f0;
  }
}

@keyframes blink {
  50% {
    opacity: 0;
  }
}

.typing-hint {
  color: #909399;
  font-size: 13px;
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

  :deep(.citation) {
    color: #409eff;
    font-size: 0.82em;
    text-decoration: none;
    vertical-align: super;
    cursor: pointer;

    &:hover {
      text-decoration: underline;
    }
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

.message-time {
  margin-top: 2px;
  max-width: 80%;
  font-size: 11px;
  color: #c0c4cc;
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

  .source-index {
    margin-right: 6px;
    color: #909399;
    font-variant-numeric: tabular-nums;
  }
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

.input-wrap {
  display: flex;
  align-items: flex-end;
  gap: 8px;

  :deep(.el-textarea) {
    flex: 1;
  }

  :deep(.el-textarea__inner) {
    border-radius: 10px;
  }
}

.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #909399;

  .empty-icon {
    font-size: 42px;
  }

  .empty-title {
    margin: 0;
    font-size: 17px;
    font-weight: 600;
    color: #606266;
  }

  .empty-sub {
    margin: 0 0 8px;
    font-size: 13px;
  }
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

    // Always show action buttons on mobile (no hover on touch)
    .delete-btn,
    .rename-btn {
      opacity: 0.6;
    }
  }

  .chat-main {
    flex: 1;
    min-height: 0; // allow flex child to shrink for keyboard
  }

  .chat-welcome {
    min-height: 180px;
    padding: 16px;
  }

  .chat-files-bar {
    padding: 8px 12px;
  }

  .message-area {
    padding: 12px;
    padding-bottom: 8px;
  }

  .message {
    padding-left: 38px;

    &.user {
      padding-right: 38px;
    }

    &::before {
      width: 28px;
      height: 28px;
      font-size: 11px;
    }
  }

  .message-bubble {
    max-width: 88%; // slightly wider on small screens
    padding: 10px 14px;
    font-size: 15px;
    border-radius: 16px;
  }

  .message-actions,
  .message-time {
    max-width: 88%;
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

    .empty-sub {
      font-size: 13px;
    }
  }

  .file-select-list {
    max-height: 60vh;
  }
}
</style>
