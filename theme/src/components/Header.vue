<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { computed, inject, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { DataTooltip } from '@/components/ui/data-tooltip'
import { useAppStore } from '@/stores/app'

const router = useRouter()
const appStore = useAppStore()

const isScrolled = inject<ReturnType<typeof ref<boolean>>>('isScrolled', ref(false))

const siteFavicon = ref('/favicon.svg?v=4')

function handleAdminClick() {
  location.href = '/admin'
}

function getInitialSitename() {
  if (typeof window !== 'undefined') {
    const injected = (window as any).__INITIAL_SITENAME__
    if (typeof injected === 'string' && injected.trim()) {
      return injected.trim()
    }
    if (document.title && document.title.trim() && document.title !== 'Monitor' && document.title !== 'Komari Monitor') {
      return document.title.trim()
    }
    try {
      const cached = localStorage.getItem('kml_cached_sitename')
      if (cached && cached.trim())
        return cached.trim()
    }
    catch {}
  }
  return 'Monitor'
}

const fallbackSitename = ref(getInitialSitename())

const sitename = computed(() => {
  const current = appStore.publicSettings?.sitename
  if (current && typeof current === 'string' && current.trim()) {
    try {
      localStorage.setItem('kml_cached_sitename', current.trim())
    }
    catch {}
    return current.trim()
  }
  return fallbackSitename.value
})
</script>

<template>
  <div
    class="transition-all duration-200 top-0 sticky z-30 border-b"
    :class="isScrolled ? 'backdrop-blur-md bg-background/80 border-border/80 shadow-xs' : 'bg-transparent border-transparent'"
  >
    <div class="px-4 flex-between h-14 max-w-[1280px] mx-auto">
      <div class="flex items-center gap-3 cursor-pointer" @click="router.push('/')">
        <Avatar class="size-7.5 rounded-lg select-none shadow-2xs">
          <AvatarImage :src="siteFavicon" :alt="sitename" class="rounded-lg object-contain" />
          <AvatarFallback class="rounded-lg">{{ sitename.slice(0, 1) }}</AvatarFallback>
        </Avatar>
        <h3 class="m-0 text-lg font-semibold">
          {{ sitename }}
        </h3>
      </div>
      <div class="flex items-center gap-2">
        <!-- 主题切换按钮 -->
        <DataTooltip
          :content="appStore.isDark ? '切换为浅色主题' : '切换为深色主题'"
          placement="left"
          content-class="whitespace-nowrap text-[11px] px-2"
        >
          <Button
            variant="ghost"
            size="icon-sm"
            class="size-8 rounded-lg border border-border/60 hover:bg-muted/60 text-muted-foreground hover:text-foreground inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
            @click="appStore.updateThemeMode(appStore.isDark ? 'light' : 'dark')"
          >
            <Icon :icon="appStore.isDark ? 'tabler:sun' : 'tabler:moon'" :width="16" :height="16" class="opacity-80" />
          </Button>
        </DataTooltip>

        <!-- 管理后台入口 -->
        <DataTooltip :content="appStore.isLoggedIn ? '进入管理后台' : '管理员登录'" placement="left" content-class="whitespace-nowrap text-[11px] px-2">
          <Button
            variant="ghost"
            size="sm"
            class="h-8 px-2.5 rounded-lg border border-border/60 hover:bg-muted/60 text-muted-foreground hover:text-foreground text-xs font-medium inline-flex items-center gap-1.5 transition-colors cursor-pointer shadow-2xs"
            @click="handleAdminClick"
          >
            <Icon icon="tabler:settings" :width="15" :height="15" class="opacity-75" />
            <span>{{ appStore.isLoggedIn ? '管理后台' : '管理' }}</span>
          </Button>
        </DataTooltip>
      </div>
    </div>
  </div>
</template>
