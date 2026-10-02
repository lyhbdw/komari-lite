<script setup lang="ts">
import type { VersionInfo } from '@/utils/api'
import { computed, onMounted, ref } from 'vue'
import { DataTooltip } from '@/components/ui/data-tooltip'
import VisitorInfoCard from '@/components/VisitorInfoCard.vue'
import { useAppStore } from '@/stores/app'
import { getSharedApi } from '@/utils/api'

const appStore = useAppStore()
const api = getSharedApi()

const serverVersion = ref<VersionInfo | null>(null)

onMounted(async () => {
  try {
    serverVersion.value = await api.getVersion()
  }
  catch {
    // 静默失败
  }
})

const formattedServerVersion = computed(() => serverVersion.value?.version ?? null)
</script>

<template>
  <VisitorInfoCard v-if="appStore.visitorInfoCardEnabled" />
  <footer class="w-full sm:flex-row sm:gap-4 max-w-[1280px] mx-auto p-4 border-t border-border/40 mt-6">
    <div class="flex flex-row items-center justify-end text-xs text-muted-foreground font-medium">
      <div class="flex gap-1 items-center">
        Powered by
        <DataTooltip
          as="span"
          placement="top"
          :content="formattedServerVersion ?? ''"
        >
          <a
            href="https://github.com/lyhbdw/komari-monitor-lite" target="_blank" rel="noopener noreferrer"
            class="transition-opacity hover:opacity-80"
          >
            <span class="font-semibold text-foreground/90 hover:text-foreground">Komari Lite</span>
          </a>
        </DataTooltip>
      </div>
    </div>
  </footer>
</template>
