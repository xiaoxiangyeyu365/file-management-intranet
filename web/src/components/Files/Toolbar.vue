<template>
  <div class="toolbar">
    <div class="toolbar-left">
      <el-button type="primary" @click="triggerUpload">
        <el-icon><Upload /></el-icon>
        上传
      </el-button>
      <el-button @click="emit('create-folder')">
        <el-icon><FolderAdd /></el-icon>
        新建文件夹
      </el-button>
    </div>

    <div class="toolbar-center">
      <el-dropdown @command="handleSortChange">
        <el-button text>
          <el-icon><Sort /></el-icon>
          {{ sortLabel }}
          <el-icon class="el-icon--right"><ArrowDown /></el-icon>
        </el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="name-asc">名称 A→Z</el-dropdown-item>
            <el-dropdown-item command="name-desc">名称 Z→A</el-dropdown-item>
            <el-dropdown-item command="size-asc">大小 小→大</el-dropdown-item>
            <el-dropdown-item command="size-desc">大小 大→小</el-dropdown-item>
            <el-dropdown-item command="updatedAt-asc">时间 旧→新</el-dropdown-item>
            <el-dropdown-item command="updatedAt-desc">时间 新→旧</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>

    <div class="toolbar-right">
      <el-button-group>
        <el-button
          :type="viewMode === 'list' ? 'primary' : ''"
          @click="setViewMode('list')"
        >
          <el-icon><List /></el-icon>
        </el-button>
        <el-button
          :type="viewMode === 'grid' ? 'primary' : ''"
          @click="setViewMode('grid')"
        >
          <el-icon><Grid /></el-icon>
        </el-button>
      </el-button-group>
    </div>

    <input
      ref="fileInputRef"
      type="file"
      multiple
      style="display: none"
      @change="handleFileSelect"
    />
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useFilesStore } from '@/stores/files'
import { useUploadStore } from '@/stores/upload'
import { Upload, FolderAdd, List, Grid, Sort, ArrowDown } from '@element-plus/icons-vue'

const filesStore = useFilesStore()
const uploadStore = useUploadStore()

const fileInputRef = ref(null)
const viewMode = ref(filesStore.viewMode)

const emit = defineEmits(['upload', 'create-folder'])

function triggerUpload() {
  fileInputRef.value?.click()
}

function handleFileSelect(event) {
  const files = event.target.files
  if (files.length > 0) {
    for (const file of files) {
      file.parentId = filesStore.currentFolder
      uploadStore.addTask(file)
    }
  }
  event.target.value = '' // Reset input
}

function setViewMode(mode) {
  if (filesStore.viewMode !== mode) {
    filesStore.toggleViewMode()
    viewMode.value = filesStore.viewMode
  }
}

const sortLabels = {
  'name-asc': '名称 A→Z',
  'name-desc': '名称 Z→A',
  'size-asc': '大小 小→大',
  'size-desc': '大小 大→小',
  'updatedAt-asc': '时间 旧→新',
  'updatedAt-desc': '时间 新→旧',
}

const sortLabel = computed(() => {
  const key = `${filesStore.sortBy}-${filesStore.sortOrder}`
  return sortLabels[key] || '排序'
})

function handleSortChange(command) {
  const [sortBy, sortOrder] = command.split('-')
  filesStore.setSort(sortBy, sortOrder)
}
</script>

<style scoped lang="scss">
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #e4e7ed;
}

.toolbar-left {
  display: flex;
  gap: 8px;
}

.toolbar-right {
  display: flex;
  gap: 8px;
}

.toolbar-center {
  display: flex;
  gap: 8px;
}
</style>