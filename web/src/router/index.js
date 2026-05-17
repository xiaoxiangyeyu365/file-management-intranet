import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import LoginView from '@/views/LoginView.vue'
import FilesView from '@/views/FilesView.vue'
import TrashView from '@/views/TrashView.vue'
import ClipboardView from '@/views/ClipboardView.vue'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: LoginView,
    meta: { guest: true }
  },
  {
    path: '/',
    name: 'Files',
    component: FilesView,
    meta: { requiresAuth: true }
  },
  {
    path: '/trash',
    name: 'Trash',
    component: TrashView,
    meta: { requiresAuth: true }
  },
  {
    path: '/clipboard',
    name: 'Clipboard',
    component: ClipboardView,
    meta: { requiresAuth: true }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()

  // Try to restore session
  if (authStore.token && !authStore.user) {
    await authStore.fetchProfile()
  }

  if (to.meta.requiresAuth && !authStore.token) {
    next('/login')
  } else if (to.meta.guest && authStore.user) {
    next('/')
  } else {
    next()
  }
})

export default router