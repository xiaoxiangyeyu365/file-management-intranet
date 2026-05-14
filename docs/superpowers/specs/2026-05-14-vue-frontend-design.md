# Vue Frontend Design - CloudBox Phase 2

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a Vue 3 web frontend for CloudBox file manager, embedded in the Go binary for single-file deployment.

**Architecture:** Vue 3 + Element Plus + Pinia + Vue Router + Vite. Frontend build output embedded via `//go:embed` and served by Go backend. Web Worker handles MD5 calculation for instant upload.

**Tech Stack:** Vue 3 (Composition API), Element Plus, Pinia, Vue Router, Vite, Axios, SparkMD5 (Web Worker)

---

## Design Decisions Summary

| Aspect | Decision |
|--------|----------|
| Integration | Embedded in Go binary |
| UI Framework | Element Plus |
| File View | Dual view (list + grid toggle) |
| Page Layout | Wide sidebar (Google Drive style) |
| Upload | Button + global drag-drop |
| Progress Display | Right side expandable panel |
| Image Preview | Modal overlay with backdrop |

---

## Project Structure

```
web/
├── src/
│   ├── App.vue                 # Root component with router-view
│   ├── main.js                 # Vue app entry, register Element Plus
│   ├── router/
│   │   └── index.js            # Route definitions
│   ├── stores/
│   │   ├── auth.js             # User state: user, token, login/logout
│   │   ├── files.js            # File list, current folder, selection
│   │   └── upload.js           # Upload queue, progress tracking
│   ├── views/
│   │   ├── LoginView.vue       # Login page (username/password form)
│   │   ├── FilesView.vue       # Main file manager interface
│   │   └── TrashView.vue       # Trash management page
│   ├── components/
│   │   ├── Layout/
│   │   │   ├── AppHeader.vue   # Top header: logo, user dropdown
│   │   │   └── AppSidebar.vue  # Left nav: 全部文件, 回收站, 设置
│   │   ├── Files/
│   │   │   ├── FileList.vue    # Table view component
│   │   │   ├── FileGrid.vue    # Grid/icon view component
│   │   │   ├── FileItem.vue    # Reusable file row/card
│   │   │   ├── Breadcrumb.vue  # Path breadcrumb navigation
│   │   │   ├── Toolbar.vue     # Upload, new folder, view toggle
│   │   │   └── SearchBar.vue   # Search input with scope selector
│   │   ├── Upload/
│   │   │   ├── UploadPanel.vue # Right side queue panel
│   │   │   └── UploadItem.vue  # Single upload progress item
│   │   ├── Preview/
│   │   │   └── ImagePreview.vue# Modal image viewer
│   │   └── Dialogs/
│   │       ├── CreateFolderDialog.vue
│   │       ├── RenameDialog.vue
│   │       ├── MoveDialog.vue
│   │       └── ConfirmDialog.vue
│   ├── utils/
│   │   ├── api.js              # Axios instance, interceptors, API calls
│   │   ├── upload.js           # Chunked upload orchestration
│   │   └── md5.worker.js       # Web Worker for MD5 calculation
│   └── styles/
│       └── main.scss           # Global styles, Element Plus overrides
├── public/
│   └── favicon.ico
├── index.html
├── vite.config.js
├── package.json
└── .env.development            # VITE_API_BASE_URL for dev proxy
```

---

## Page Layout

### FilesView (Main Page)

```
┌─────────────────────────────────────────────────────────────────┐
│  [Logo] CloudBox                    [用户名 ▼]                  │  ← AppHeader
├────────────┬────────────────────────────────────────────────────┤
│            │  [面包屑: 根目录 > 文档]  [🔍 搜索...]             │
│  全部文件   │  [📤 上传] [新建文件夹] [列表|图标]               │  ← Toolbar
│  回收站     ├────────────────────────────────────────────────────┤
│  设置       │                                                    │
│            │   ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐               │
│            │   │ 📁  │  │ 📁  │  │ 📄  │  │ 📄  │               │  ← FileGrid
│            │   │文档  │  │图片  │  │报告  │  │数据  │               │    or FileList
│            │   └─────┘  └─────┘  └─────┘  └─────┘               │
│            │                                                    │
└────────────┴────────────────────────────────────────────────────┘
```

### Upload Panel (Collapsed/Expanded)

```
Collapsed:  [上传进度 (2) ▼]                    ← Click to expand

Expanded:
┌─────────────────────────────────┐
│ 上传队列                    [✕] │
├─────────────────────────────────┤
│ 📄 视频文件.mp4                  │
│ ████████████░░░░░░░░ 45%        │
│ 15MB / 33MB • 2.1 MB/s   [取消] │
├─────────────────────────────────┤
│ 📄 文档.pdf                      │
│ 等待中...                  [取消]│
└─────────────────────────────────┘
```

### Image Preview Modal

```
┌─────────────────────────────────────────────────────────────────┐
│                         [✕]                                     │
│                                                                 │
│                    ┌───────────────┐                           │
│                    │               │                           │
│                    │     🖼️       │                           │
│                    │               │                           │
│                    └───────────────┘                           │
│                                                                 │
│                      照片.jpg                                   │
│                   1920 × 1080 • 2.3 MB                          │
└─────────────────────────────────────────────────────────────────┘
```

---

## Component Specifications

### AppHeader.vue

**Props:** None
**State:** From auth store (user)
**Features:**
- Logo and app name on left
- User dropdown on right: 修改密码, 退出登录
- Responsive: collapse on mobile

### AppSidebar.vue

**Props:** None
**State:** Current route from router
**Features:**
- Navigation items: 全部文件, 回收站, 设置
- Active item highlighted
- Collapse to icons on narrow screens

### Toolbar.vue

**Props:** None
**Emits:** upload, create-folder, toggle-view
**Features:**
- Upload button (triggers file input)
- New folder button
- View toggle (list/grid icons)
- Disabled during search

### Breadcrumb.vue

**Props:** `path: Array<{ id: number, name: string }>`
**Emits:** `navigate(id: number)`
**Features:**
- Clickable path segments
- Root folder icon
- Overflow: truncate middle with ellipsis

### SearchBar.vue

**Props:** None
**State:** Search keyword, scope (全局/当前文件夹)
**Features:**
- Text input with search icon
- Scope dropdown (全局搜索 / 在当前文件夹搜索)
- Sort selector (相关度/时间/名称)
- Clear button
- Auto-search on Enter or 300ms debounce

### FileList.vue (Table View)

**Props:** `files: File[]`
**Emits:** select, open, preview, contextmenu
**Features:**
- Element Plus el-table
- Columns: checkbox, icon, name, size, modified, actions
- Row click: select
- Row double-click: open (folder) or preview (file)
- Right-click: context menu (rename, download, delete, move)
- Sortable columns

### FileGrid.vue (Icon View)

**Props:** `files: File[]`
**Emits:** select, open, preview, contextmenu
**Features:**
- CSS Grid layout, responsive columns
- File cards: icon thumbnail, name, size
- Folder cards: folder icon, name
- Click: select
- Double-click: open or preview
- Right-click: context menu

### UploadPanel.vue

**Props:** None
**State:** From upload store
**Features:**
- Collapsible panel (toggle button shows count)
- List of UploadItem components
- Overall progress summary
- Clear completed button

### UploadItem.vue

**Props:** `task: UploadTask`
**Emits:** cancel, retry
**Features:**
- File icon + name
- Progress bar with percentage
- Speed and ETA
- Cancel button (during upload)
- Retry button (on error)
- Status indicators: uploading, completed, error

### ImagePreview.vue

**Props:** `visible: boolean`, `file: File`
**Emits:** close
**Features:**
- el-dialog with dark overlay
- Image centered, max-width 90vw, max-height 80vh
- Filename below image
- Close on: click overlay, X button, Escape key
- Future: zoom controls, prev/next navigation

---

## State Management

### auth.js (Pinia Store)

```javascript
state: {
  user: null,           // { id, username, role }
  token: null,          // JWT token
}

actions: {
  async login(username, password)
  logout()
  async changePassword(oldPassword, newPassword)
  checkAuth()           // Check localStorage for persisted token
}
```

### files.js (Pinia Store)

```javascript
state: {
  files: [],            // Current folder's files
  currentFolder: 0,     // Current folder ID (0 = root)
  selectedIds: [],      // Selected file IDs
  viewMode: 'grid',     // 'list' | 'grid'
  loading: false,
  searchKeyword: '',
  searchScope: null,    // null = global, number = folder ID
  searchSort: 'relevance', // 'relevance' | 'time' | 'name'
  searchResults: [],
}

actions: {
  async fetchFiles(folderId)
  async searchFiles(keyword, folderId, sort)
  async createFolder(parentId, name)
  async renameFile(fileId, newName)
  async deleteFile(fileId)
  async moveFiles(fileIds, targetFolderId)
  async downloadFile(fileId)
  async downloadFolder(folderId)
  toggleViewMode()
  setSelected(ids)
}
```

### upload.js (Pinia Store)

```javascript
state: {
  tasks: [],            // UploadTask[]
  expanded: true,       // Panel expanded state
}

// UploadTask: { id, file, md5, status, progress, speed, error, uploadId }

actions: {
  addTask(file)
  removeTask(taskId)
  async startUpload(taskId)   // Init, chunk upload, complete
  cancelUpload(taskId)
  retryUpload(taskId)
  setExpanded(expanded)
}
```

---

## API Utilities

### api.js

```javascript
// Axios instance with interceptors
const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
});

// Request interceptor: add Authorization header
api.interceptors.request.use(config => {
  const token = useAuthStore().token;
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

// Response interceptor: handle 401, errors
api.interceptors.response.use(
  response => response.data,
  error => {
    if (error.response?.status === 401) {
      useAuthStore().logout();
      router.push('/login');
    }
    return Promise.reject(error);
  }
);

// API functions
export const authAPI = {
  login: (username, password) => api.post('/auth/login', { username, password }),
  changePassword: (oldPassword, newPassword) => api.post('/auth/password', { oldPassword, newPassword }),
};

export const fileAPI = {
  list: (folderId) => api.get('/files', { params: { folderId } }),
  search: (keyword, folderId, sort) => api.get('/files/search', { params: { keyword, folderId, sort } }),
  lookup: (parentId, name) => api.get('/files/lookup', { params: { parentId, name } }),
  rename: (id, name) => api.put(`/files/${id}`, { name }),
  delete: (id) => api.delete(`/files/${id}`),
  move: (fileIds, targetFolderId) => api.patch('/files/move', { fileIds, targetFolderId }),
  download: (id) => `/api/files/${id}/download`,
};

export const folderAPI = {
  create: (parentId, name) => api.post('/folders', { parentId, name }),
  download: (id) => `/api/folders/${id}/download`,
};

export const uploadAPI = {
  init: (md5, name, parentId, size) => api.post('/upload/init', { md5, name, parentId, size }),
  uploadChunk: (uploadId, index, chunk) => {
    const formData = new FormData();
    formData.append('chunk', chunk);
    return api.put(`/upload/${uploadId}/chunk/${index}`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    });
  },
  progress: (uploadId) => api.get(`/upload/${uploadId}/progress`),
  complete: (uploadId, name, parentId) => api.post(`/upload/${uploadId}/complete`, { name, parentId }),
};

export const trashAPI = {
  list: () => api.get('/trash'),
  restore: (id) => api.post(`/trash/${id}/restore`),
  delete: (id) => api.delete(`/trash/${id}`),
};

export const previewAPI = {
  get: (id) => api.get(`/preview/${id}`),
};
```

---

## Upload Flow

### Sequence

1. User selects files via button or drag-drop
2. For each file:
   a. Create UploadTask with status 'hashing'
   b. Send to Web Worker for MD5 calculation
   c. Worker returns MD5 hash
3. Call `/upload/init` with MD5
4. If response.uploaded === true:
   - Instant upload complete, show success
   - Refresh file list
5. Else:
   - Store uploadId
   - Split file into 5MB chunks
   - Upload chunks concurrently (max 3 at a time)
   - Update progress bar on each chunk complete
   - When all chunks done, call `/upload/complete`
   - Refresh file list

### Error Handling

- Network error: Retry button appears
- Server error (5xx): Show error message, retry
- Conflict (same name): Auto-rename with (1), (2) suffix
- Cancelled: Remove task or mark as cancelled

### Web Worker (md5.worker.js)

```javascript
import SparkMD5 from 'spark-md5';

self.onmessage = function(e) {
  const { file, taskId } = e.data;
  const chunkSize = 2 * 1024 * 1024; // 2MB chunks for hashing
  const chunks = Math.ceil(file.size / chunkSize);
  const spark = new SparkMD5.ArrayBuffer();
  let currentChunk = 0;

  const reader = new FileReader();

  reader.onload = function(e) {
    spark.append(e.target.result);
    currentChunk++;

    if (currentChunk < chunks) {
      loadNext();
    } else {
      const md5 = spark.end();
      self.postMessage({ taskId, md5 });
    }
  };

  function loadNext() {
    const start = currentChunk * chunkSize;
    const end = Math.min(start + chunkSize, file.size);
    reader.readAsArrayBuffer(file.slice(start, end));
  }

  loadNext();
};
```

---

## Route Definitions

```javascript
const routes = [
  { path: '/login', name: 'Login', component: LoginView, meta: { guest: true } },
  { path: '/', name: 'Files', component: FilesView, meta: { requiresAuth: true } },
  { path: '/trash', name: 'Trash', component: TrashView, meta: { requiresAuth: true } },
];

// Navigation guard
router.beforeEach((to, from, next) => {
  const authStore = useAuthStore();

  if (to.meta.requiresAuth && !authStore.token) {
    next('/login');
  } else if (to.meta.guest && authStore.token) {
    next('/');
  } else {
    next();
  }
});
```

---

## Build Configuration

### vite.config.js

```javascript
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  base: '/',  // Embedded mode uses root
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: false,
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});
```

### package.json

```json
{
  "name": "cloudbox-web",
  "version": "1.0.0",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "vue": "^3.4.0",
    "vue-router": "^4.3.0",
    "pinia": "^2.1.0",
    "element-plus": "^2.6.0",
    "axios": "^1.6.0",
    "spark-md5": "^3.0.2"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.0.0",
    "vite": "^5.0.0",
    "sass": "^1.70.0"
  }
}
```

---

## Go Backend Integration

### embed.go (at project root)

```go
package main

import "embed"

//go:embed web/dist
var staticFiles embed.FS
```

### cmd/server/main.go (route setup)

```go
// Serve static files (must be after API routes)
r.Static("/assets", "./web/dist/assets")
r.NoRoute(func(c *gin.Context) {
    data, _ := staticFiles.ReadFile("web/dist/index.html")
    c.Data(200, "text/html; charset=utf-8", data)
})
```

---

## Testing Checklist

- [ ] Login flow: correct credentials → redirect to files
- [ ] Login flow: wrong credentials → show error
- [ ] File list: load root folder
- [ ] File list: navigate into subfolder
- [ ] Breadcrumb: click segment to navigate
- [ ] Create folder: success → refresh list
- [ ] Rename: success → update in list
- [ ] Delete: confirm → move to trash
- [ ] Upload: small file (< 5MB) → instant or single chunk
- [ ] Upload: large file → chunked with progress
- [ ] Upload: drag-drop → trigger upload
- [ ] Download: single file → browser downloads
- [ ] Download: folder → ZIP download
- [ ] Preview: image file → modal shows
- [ ] Search: keyword → results appear
- [ ] Search: scope selector → filter results
- [ ] Trash: list deleted files
- [ ] Trash: restore → file reappears in list
- [ ] Trash: permanent delete → confirm → remove
- [ ] Responsive: narrow screen → sidebar collapses
