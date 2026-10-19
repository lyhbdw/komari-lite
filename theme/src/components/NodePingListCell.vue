<script setup lang="ts">
import NodePingSummary from '@/components/NodePingSummary.vue'

const props = defineProps<{
  uuid: string
  online: boolean
}>()
</script>

<template>
  <NodePingSummary :uuid="props.uuid" compact />
</template>
