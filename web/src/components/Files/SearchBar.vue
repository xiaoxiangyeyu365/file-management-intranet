<template>
  <div class="search-bar">
    <el-input
      v-model="keyword"
      placeholder="搜索文件..."
      clearable
      @input="handleSearch"
      @keyup.enter="handleSearch"
    >
      <template #prefix>
        <el-icon><Search /></el-icon>
      </template>
    </el-input>

    <el-select
      v-model="scope"
      placeholder="搜索范围"
      style="width: 140px; margin-left: 8px"
      @change="handleSearch"
    >
      <el-option label="全局搜索" :value="null" />
      <el-option label="当前文件夹" :value="currentFolder" />
    </el-select>

    <el-select
      v-model="sort"
      placeholder="排序"
      style="width: 120px; margin-left: 8px"
      @change="handleSearch"
    >
      <el-option label="相关度" value="relevance" />
      <el-option label="时间" value="time" />
      <el-option label="名称" value="name" />
    </el-select>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useFilesStore } from '@/stores/files'
import { Search } from '@element-plus/icons-vue'

const filesStore = useFilesStore()

const keyword = ref('')
const scope = ref(null)
const sort = ref('relevance')
const currentFolder = ref(0)

let debounceTimer = null

function handleSearch() {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    if (keyword.value.trim()) {
      filesStore.searchFiles(keyword.value, scope.value, sort.value)
    } else {
      filesStore.searchResults = []
      filesStore.isSearching = false
    }
  }, 300)
}

watch(() => filesStore.currentFolder, (newVal) => {
  currentFolder.value = newVal
  scope.value = null
})
</script>

<style scoped lang="scss">
.search-bar {
  display: flex;
  align-items: center;
  margin-left: auto;
}
</style>