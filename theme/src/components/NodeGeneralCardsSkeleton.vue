<script setup lang="ts">
import { CardX } from '@/components/ui/card-x'
import { Skeleton } from '@/components/ui/skeleton'
</script>

<template>
  <div class="p-4 flex flex-col gap-4">
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
      <CardX v-for="i in 4" :key="i" class="h-full border border-border/70 bg-card rounded-lg p-3 shadow-xs">
        <div class="flex flex-col justify-between gap-3 h-full">
          <div class="flex items-center justify-between">
            <Skeleton class="h-3 w-16" />
            <Skeleton class="size-5 rounded-md" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Skeleton class="h-6 w-24" />
            <Skeleton class="h-2.5 w-32" />
          </div>
        </div>
      </CardX>
    </div>
  </div>
</template>
