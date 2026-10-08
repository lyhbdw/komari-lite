<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { computed, inject, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { DataTooltip } from '@/components/ui/data-tooltip'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()

const isScrolled = inject<ReturnType<typeof ref<boolean>>>('isScrolled', ref(false))

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
      <div class="flex items-center gap-3 select-none">
        <div class="size-7.5 rounded-lg select-none text-foreground shrink-0 flex items-center justify-center">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="none" class="size-full">
            <rect x="36" y="36" width="440" height="440" rx="96" stroke="currentColor" stroke-width="40" fill="none" />
            <path d="M 148 168 L 246 256 L 148 344" stroke="currentColor" stroke-width="46" stroke-linecap="round" stroke-linejoin="round" />
            <rect x="276" y="322" width="112" height="44" rx="12" fill="currentColor" />
          </svg>
        </div>
        <h3 class="m-0 text-lg font-semibold">
          {{ sitename }}
        </h3>
      </div>
      <div class="flex items-center gap-2">
        <!-- 主题切换下拉菜单 -->
        <DropdownMenu>
          <DataTooltip
            content="主题外观设置"
            placement="bottom"
            content-class="whitespace-nowrap text-[11px] px-2"
          >
            <DropdownMenuTrigger as-child>
              <Button
                variant="ghost"
                size="icon-sm"
                class="size-8 rounded-lg border border-border/60 hover:bg-muted/60 text-muted-foreground hover:text-foreground inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
                aria-label="切换主题"
              >
                <Icon
                  :icon="appStore.themeMode === 'auto' ? 'tabler:device-desktop' : appStore.isDark ? 'tabler:moon' : 'tabler:sun'"
                  :width="16" :height="16" class="opacity-80"
                />
              </Button>
            </DropdownMenuTrigger>
          </DataTooltip>
          <DropdownMenuContent align="end" class="w-36">
            <DropdownMenuItem
              class="flex items-center justify-between"
              @click="appStore.updateThemeMode('light')"
            >
              <span class="flex items-center gap-2">
                <Icon icon="tabler:sun" :width="14" :height="14" class="opacity-70" />
                <span>浅色模式</span>
              </span>
              <Icon v-if="appStore.themeMode === 'light'" icon="tabler:check" :width="14" :height="14" class="text-primary" />
            </DropdownMenuItem>
            <DropdownMenuItem
              class="flex items-center justify-between"
              @click="appStore.updateThemeMode('dark')"
            >
              <span class="flex items-center gap-2">
                <Icon icon="tabler:moon" :width="14" :height="14" class="opacity-70" />
                <span>深色模式</span>
              </span>
              <Icon v-if="appStore.themeMode === 'dark'" icon="tabler:check" :width="14" :height="14" class="text-primary" />
            </DropdownMenuItem>
            <DropdownMenuItem
              class="flex items-center justify-between"
              @click="appStore.updateThemeMode('auto')"
            >
              <span class="flex items-center gap-2">
                <Icon icon="tabler:device-desktop" :width="14" :height="14" class="opacity-70" />
                <span>跟随系统</span>
              </span>
              <Icon v-if="appStore.themeMode === 'auto'" icon="tabler:check" :width="14" :height="14" class="text-primary" />
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

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
