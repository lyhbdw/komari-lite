<script setup lang="ts">
import { CardX } from '@/components/ui/card-x'
import { Skeleton } from '@/components/ui/skeleton'

defineProps<{
  count?: number
}>()
</script>

<template>
  <div class="gap-4 grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3">
    <CardX
      v-for="i in (count || 3)"
      :key="i"
      size="small"
      class="border border-border/80 rounded-lg bg-card p-3 shadow-xs"
    >
      <div class="flex flex-col gap-3 h-56 justify-between">
        <div class="flex items-center justify-between">
          <Skeleton class="h-4 w-20" />
          <Skeleton class="h-3.5 w-12" />
        </div>
        <div class="flex-1 flex items-end gap-1.5 pt-4">
          <Skeleton
            v-for="j in 12"
            :key="j"
            class="flex-1 rounded-xs"
            :style="{ height: `${20 + ((j * 17) % 70)}%` }"
          />
        </div>
      </div>
    </CardX>
  </div>
</template>
