<script setup lang="ts">
import { CardX } from '@/components/ui/card-x'
import { Skeleton } from '@/components/ui/skeleton'
</script>

<template>
  <CardX size="small" header-class="items-start" class="h-full w-full rounded-xl border border-border/70 bg-card shadow-xs">
    <template #header>
      <div class="flex min-w-0 flex-col gap-1.5">
        <div class="flex min-w-0 items-center gap-2">
          <Skeleton class="size-2 shrink-0 rounded-full" />
          <Skeleton class="h-4 w-24 rounded sm:w-28" />
        </div>
        <div class="flex flex-wrap items-center gap-1.5">
          <Skeleton class="h-4.5 w-20 rounded-md" />
        </div>
      </div>
    </template>

    <template #header-extra>
      <div class="flex h-5 items-center gap-1.5">
        <Skeleton class="size-3.5 rounded" />
        <Skeleton class="size-3.5 rounded-[2px]" />
      </div>
    </template>

    <template #default>
      <div class="flex flex-col gap-2">
        <!-- 四大核心指标骨架 -->
        <div class="gap-x-3 gap-y-2 grid grid-cols-2">
          <div v-for="i in 4" :key="i" class="flex flex-col gap-1">
            <div class="flex justify-between items-center">
              <Skeleton class="h-2.5 w-7" />
              <Skeleton class="h-2.5 w-9" />
            </div>
            <Skeleton class="h-[3.5px] w-full rounded-full my-0.5" />
            <Skeleton class="h-2 w-14" />
          </div>
        </div>

        <!-- 下半区骨架 -->
        <div class="flex flex-col gap-y-2 pt-2.5 border-t border-border/50">
          <div class="flex justify-between items-center min-h-[18px]">
            <Skeleton class="h-2.5 w-8" />
            <div class="flex gap-2">
              <Skeleton class="h-2.5 w-12" />
              <Skeleton class="h-2.5 w-12" />
            </div>
          </div>
          <div class="flex justify-between items-center min-h-[18px]">
            <Skeleton class="h-2.5 w-8" />
            <Skeleton class="h-2.5 w-14" />
          </div>
          <div class="flex justify-between items-center min-h-[18px]">
            <Skeleton class="h-2.5 w-8" />
            <Skeleton class="h-2.5 w-24" />
          </div>
          <div class="flex justify-between items-center min-h-[18px]">
            <Skeleton class="h-2.5 w-8" />
            <Skeleton class="h-2.5 w-20" />
          </div>
          <div class="grid grid-cols-2 gap-3 pt-0.5">
            <div class="flex flex-col gap-1">
              <div class="flex justify-between">
                <Skeleton class="h-2 w-6" />
                <Skeleton class="h-2 w-8" />
              </div>
              <Skeleton class="h-4 w-full rounded-[2px]" />
            </div>
            <div class="flex flex-col gap-1">
              <div class="flex justify-between">
                <Skeleton class="h-2 w-6" />
                <Skeleton class="h-2 w-8" />
              </div>
              <Skeleton class="h-4 w-full rounded-[2px]" />
            </div>
          </div>
        </div>
      </div>
    </template>
  </CardX>
</template>
