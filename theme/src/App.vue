<script setup lang="ts">
import { nextTick, onMounted, onUnmounted } from 'vue'
import { Toaster } from '@/components/ui/sonner'
import { destroyInitManager, initApp } from '@/utils/init'
import Footer from './components/Footer.vue'
import Header from './components/Header.vue'
import Provider from './components/Provider.vue'

onMounted(async () => {
  try {
    await initApp()
    await nextTick()
  }
  catch (error) {
    console.error('[App] Initialization failed:', error)
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
