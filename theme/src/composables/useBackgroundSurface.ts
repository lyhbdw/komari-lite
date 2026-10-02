import { computed } from 'vue'

export function useBackgroundSurface() {
  const hasCustomBackground = computed(() => false)

  function pickSurfaceClass(defaultClass: string, _customBackgroundClass: string): string {
    return defaultClass
  }

  return {
    hasCustomBackground,
    pickSurfaceClass,
  }
}
