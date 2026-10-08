<script setup lang="ts">
import type { DropdownMenuSeparatorProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { DropdownMenuSeparator } from 'reka-ui'
import { cn } from '@/lib/utils'

interface Props extends DropdownMenuSeparatorProps {
  class?: HTMLAttributes['class']
}

const props = defineProps<Props>()
</script>

<template>
  <DropdownMenuSeparator
    v-bind="props"
    :class="cn('-mx-1 my-1 h-px bg-border/60', props.class)"
  />
</template>
