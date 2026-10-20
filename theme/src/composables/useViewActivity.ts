import type { InjectionKey, Ref } from 'vue'
import { getCurrentInstance, inject, onActivated, onDeactivated, provide, readonly, ref } from 'vue'

const viewActivityKey: InjectionKey<Readonly<Ref<boolean>>> = Symbol('viewActivity')
const defaultActivity = readonly(ref(true))

export function provideViewActivity(): void {
  const active = ref(true)
  onActivated(() => {
    active.value = true
  })
  onDeactivated(() => {
    active.value = false
  })
  // New descendants must inherit a cached view's current activity immediately.
  provide(viewActivityKey, readonly(active))
}

export function useViewActivity(): Readonly<Ref<boolean>> {
  return getCurrentInstance() ? inject(viewActivityKey, defaultActivity) : defaultActivity
}
