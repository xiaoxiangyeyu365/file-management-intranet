import { defineStore } from 'pinia'
import { uploadAPI } from '@/utils/api'
import { useFilesStore } from './files'
import MD5Worker from '@/utils/md5.worker.js?worker'
import { ElMessageBox } from 'element-plus'

const CHUNK_SIZE = 5 * 1024 * 1024 // 5MB
const MAX_CONCURRENT = 3

export const useUploadStore = defineStore('upload', {
  state: () => ({
    tasks: [],
    expanded: localStorage.getItem('cloudbox_uploadExpanded') === 'true',
    worker: null,
    concurrent: 0
  }),

  getters: {
    activeTasks: (state) => state.tasks.filter(t => t.status === 'uploading'),
    completedTasks: (state) => state.tasks.filter(t => t.status === 'completed'),
    failedTasks: (state) => state.tasks.filter(t => t.status === 'error'),
    pendingTasks: (state) => state.tasks.filter(t => t.status === 'pending' || t.status === 'hashing' || t.status === 'confirming'),
    totalCount: (state) => state.tasks.length,
    activeCount: (state) => state.tasks.filter(t => t.status === 'uploading').length
  },

  actions: {
    initWorker() {
      if (!this.worker) {
        this.worker = new MD5Worker()
        this.worker.onmessage = (e) => {
          const { taskId, md5 } = e.data
          this.onMD5Calculated(taskId, md5)
        }
      }
    },

    addTask(file) {
      this.initWorker()
      const taskId = Date.now().toString() + Math.random().toString(36).substr(2, 9)
      const task = {
        id: taskId,
        file,
        name: file.name,
        size: file.size,
        status: 'hashing',
        progress: 0,
        speed: 0,
        uploadedBytes: 0,
        error: null,
        uploadId: null,
        md5: null,
        existingFile: null
      }
      this.tasks.push(task)
      this.worker.postMessage({ taskId, file })
      return taskId
    },

    async onMD5Calculated(taskId, md5) {
      const task = this.tasks.find(t => t.id === taskId)
      if (!task) return

      task.md5 = md5

      // Check if file already exists (by name in current folder)
      const filesStore = useFilesStore()
      const existing = await filesStore.checkFileExists(task.name, filesStore.currentFolder)

      if (existing) {
        task.existingFile = existing
        task.status = 'confirming'

        try {
          await ElMessageBox.confirm(
            `文件 "${task.name}" 已存在。\n\n请选择操作：\n• 覆盖：将重新上传并替换现有文件\n• 跳过：取消本次上传\n• 重命名：自动添加数字后缀上传`,
            '文件已存在',
            {
              confirmButtonText: '覆盖',
              cancelButtonText: '跳过',
              distinguishCancelAndClose: true,
              type: 'warning',
              showInput: false
            }
          )
          // User chose to overwrite - remove existing file first
          await filesStore.deleteFile(existing.id)
          this.processUpload(task)
        } catch (action) {
          // User cancelled or chose skip
          task.status = 'cancelled'
          this.removeTask(taskId)
        }
      } else {
        this.processUpload(task)
      }
    },

    async processUpload(task) {
      try {
        task.status = 'pending'
        const response = await uploadAPI.init(task.md5, task.name, task.file.parentId || 0, task.size)

        // Backend returns {code:0, data:{uploadID, instant, chunkSize}}
        const data = response?.data || response

        if (data.instant) {
          task.status = 'completed'
          task.progress = 100
          this.refreshFiles()
          return
        }

        task.uploadId = data.uploadID
        task.status = 'uploading'
        await this.uploadChunks(task)
      } catch (error) {
        task.status = 'error'
        task.error = error.message || error.response?.data?.message || 'Upload failed'
      }
    },

    async uploadChunks(task) {
      const totalChunks = Math.ceil(task.size / CHUNK_SIZE)
      let completedChunks = 0
      const startTime = Date.now()

      while (completedChunks < totalChunks) {
        if (task.status === 'cancelled') break

        while (this.concurrent >= MAX_CONCURRENT) {
          await new Promise(resolve => setTimeout(resolve, 100))
          if (task.status === 'cancelled') break
        }

        if (task.status === 'cancelled') break

        const chunkIndex = completedChunks
        this.concurrent++

        const start = chunkIndex * CHUNK_SIZE
        const end = Math.min(start + CHUNK_SIZE, task.size)
        const chunk = task.file.slice(start, end)

        try {
          await uploadAPI.uploadChunk(task.uploadId, chunkIndex, chunk)
          completedChunks++

          const elapsed = (Date.now() - startTime) / 1000
          task.uploadedBytes = end
          task.progress = Math.round((completedChunks / totalChunks) * 100)
          task.speed = Math.round(task.uploadedBytes / elapsed)
        } catch (error) {
          this.concurrent--
          throw error
        }

        this.concurrent--
      }

      if (task.status !== 'cancelled' && completedChunks === totalChunks) {
        await uploadAPI.complete(task.uploadId, task.name, task.file.parentId || 0, task.md5, task.size)
        task.status = 'completed'
        task.progress = 100
        this.refreshFiles()
      }
    },

    cancelUpload(taskId) {
      const task = this.tasks.find(t => t.id === taskId)
      if (!task) return

      if (task.uploadId && task.status === 'uploading') {
        uploadAPI.cancel(task.uploadId).catch(() => {})
      }
      task.status = 'cancelled'
      this.removeTask(taskId)
    },

    retryUpload(taskId) {
      const task = this.tasks.find(t => t.id === taskId)
      if (!task) return

      task.status = 'hashing'
      task.error = null
      task.progress = 0
      this.worker.postMessage({ taskId: task.id, file: task.file })
    },

    removeTask(taskId) {
      this.tasks = this.tasks.filter(t => t.id !== taskId)
    },

    clearCompleted() {
      this.tasks = this.tasks.filter(t => t.status !== 'completed')
    },

    setExpanded(expanded) {
      this.expanded = expanded
      localStorage.setItem('cloudbox_uploadExpanded', expanded.toString())
    },

    refreshFiles() {
      const filesStore = useFilesStore()
      filesStore.fetchFiles(filesStore.currentFolder)
    }
  }
})