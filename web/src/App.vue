<template>
  <router-view />
  <MobileTabBar v-if="authStore.isLoggedIn" />
</template>

<script setup>
import { useAuthStore } from '@/stores/auth'
import MobileTabBar from '@/components/Layout/MobileTabBar.vue'

const authStore = useAuthStore()
authStore.fetchProfile()
</script>

<style>
#app {
  height: 100%;
}
</style>