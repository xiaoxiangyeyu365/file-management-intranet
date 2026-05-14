<template>
  <router-view />
</template>

<script setup>
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
authStore.fetchProfile()
</script>

<style>
#app {
  height: 100%;
}
</style>