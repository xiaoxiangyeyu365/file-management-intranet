<template>
  <el-dialog
    v-model="visible"
    title="移动到"
    width="500px"
    @closed="handleClosed"
  >
    <div class="folder-tree">
      <div
        class="folder-item"
        :class="{ active: targetFolderId === 0 }"
        @click="targetFolderId = 0"
      >
        <el-icon><House /></el-icon>
        根目录
      </div>

      <el-tree
        :data="folderTreeData"
        :props="{ label: 'name', children: 'children' }"
        node-key="id"
        :expand-on-click-node="false"
        :current-node-key="targetFolderId"
        v-loading="treeLoading"
        @node-click="handleNodeClick"
      >
        <template #default="{ node, data }">
          <span class="folder-node">
            <el-icon><Folder /></el-icon>
            <span>{{ data.name }}</span>
          </span>
        </template>
      </el-tree>
    </div>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="loading" @click="handleSubmit">移动</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useFilesStore } from '@/stores/files'
import { folderAPI } from '@/utils/api'
import { ElMessage } from 'element-plus'
import { House, Folder } from '@element-plus/icons-vue'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  files: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['update:modelValue', 'moved'])

const filesStore = useFilesStore()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const targetFolderId = ref(0)
const loading = ref(false)
const folderTreeData = ref([])
const treeLoading = ref(false)

// Build folder tree from API response
async function fetchFolderTree() {
  treeLoading.value = true
  try {
    const res = await folderAPI.listTree()
    folderTreeData.value = res?.data || res || []
  } catch (err) {
    console.error('Failed to load folder tree:', err)
    folderTreeData.value = []
  } finally {
    treeLoading.value = false
  }
}

watch(visible, (val) => {
  if (val) {
    targetFolderId.value = 0
    fetchFolderTree()
  }
})

function handleNodeClick(data) {
  if (props.files.some(f => f.id === data.id)) {
    ElMessage.warning('不能移动到自身或子文件夹')
    return
  }
  targetFolderId.value = data.id
}

async function handleSubmit() {
  if (props.files.length === 0) return

  loading.value = true
  try {
    await filesStore.moveFiles(props.files.map(f => f.id), targetFolderId.value)
    ElMessage.success('移动成功')
    emit('moved')
    visible.value = false
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '移动失败')
  } finally {
    loading.value = false
  }
}

function handleClosed() {
  targetFolderId.value = 0
  folderTreeData.value = []
}
</script>

<style scoped lang="scss">
.folder-tree {
  max-height: 400px;
  overflow-y: auto;

  .folder-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    cursor: pointer;
    border-radius: 4px;
    transition: background 0.2s;

    &:hover {
      background: #f5f7fa;
    }

    &.active {
      background: #ecf5ff;
      color: #409eff;
    }
  }

  .folder-node {
    display: flex;
    align-items: center;
    gap: 8px;
  }
}
</style>