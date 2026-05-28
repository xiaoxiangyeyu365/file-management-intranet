import { ref, onMounted, onUnmounted } from 'vue'

export function useResponsive() {
  const isMobile = ref(false)

  let mediaQuery = null
  let handler = null

  onMounted(() => {
    mediaQuery = window.matchMedia('(max-width: 767px)')
    isMobile.value = mediaQuery.matches

    handler = (e) => {
      isMobile.value = e.matches
    }
    mediaQuery.addEventListener('change', handler)
  })

  onUnmounted(() => {
    if (mediaQuery && handler) {
      mediaQuery.removeEventListener('change', handler)
    }
  })

  return { isMobile }
}
