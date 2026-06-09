import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import LoginView from '@/views/LoginView.vue'
import FilesView from '@/views/FilesView.vue'
import TrashView from '@/views/TrashView.vue'
import ClipboardView from '@/views/ClipboardView.vue'
import RegisterView from '@/views/RegisterView.vue'
import AdminUsersView from '@/views/AdminUsersView.vue'
import AdminAuditView from '@/views/AdminAuditView.vue'
import ForcePasswordChangeView from '@/views/ForcePasswordChangeView.vue'
const ChatView = () => import('../views/ChatView.vue')

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: LoginView,
    meta: { guest: true }
  },
  {
    path: '/register',
    name: 'Register',
    component: RegisterView,
    meta: { guest: true }
  },
  {
    path: '/change-password',
    name: 'ForcePasswordChange',
    component: ForcePasswordChangeView,
    meta: { requiresAuth: true }
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
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: AdminUsersView,
    meta: { requiresAuth: true, requiresAdmin: true }
  },
  {
    path: '/admin/audit',
    name: 'AdminAudit',
    component: AdminAuditView,
    meta: { requiresAuth: true, requiresAdmin: true }
  },
  {
    path: '/chat',
    name: 'Chat',
    component: ChatView,
    meta: { requiresAuth: true }
  },
  {
    path: '/chat/:id',
    name: 'ChatConversation',
    component: ChatView,
    meta: { requiresAuth: true }
  },
  {
    path: '/s/:token',
    name: 'SharePreview',
    component: () => import('../views/SharePreview.vue'),
    meta: { public: true }
  },
  {
    path: '/shares',
    name: 'Shares',
    component: () => import('../views/SharesView.vue'),
    meta: { requiresAuth: true }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  if (to.meta.public) {
    return next()
  }

  const authStore = useAuthStore()

  if (authStore.token && !authStore.user) {
    await authStore.fetchProfile()
  }

  if (to.meta.requiresAuth && !authStore.token) {
    next('/login')
  } else if (authStore.requirePasswordChange && to.name !== 'ForcePasswordChange') {
    next('/change-password')
  } else if (to.meta.requiresAdmin && !authStore.isAdmin) {
    next('/')
  } else if (to.meta.guest && authStore.user) {
    next('/')
  } else {
    next()
  }
})

export default router
