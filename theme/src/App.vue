<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from 'vue'
import { Toaster } from '@/components/ui/sonner'
import { useAppStore } from '@/stores/app'
import { destroyInitManager, initApp } from '@/utils/init'
import Footer from './components/Footer.vue'
import Header from './components/Header.vue'
import LoadingCover from './components/LoadingCover.vue'
import Provider from './components/Provider.vue'

const appStore = useAppStore()

const isReady = ref(false)

onMounted(async () => {
  try {
    await initApp()
    await nextTick()
    isReady.value = true
  }
  catch (error) {
    console.error('[App] Initialization failed:', error)
    isReady.value = true
  }
})

onUnmounted(() => {
  destroyInitManager()
})
</script>

<template>
  <Provider>
    <Header />
    <main class="flex-1">
      <div class="max-w-[1280px] mx-auto">
        <RouterView v-slot="{ Component }">
          <KeepAlive :include="['HomeView']">
            <component :is="Component" />
          </KeepAlive>
        </RouterView>
      </div>
    </main>
    <Footer />
    <Toaster rich-colors close-button position="top-center" />
  </Provider>
</template>
