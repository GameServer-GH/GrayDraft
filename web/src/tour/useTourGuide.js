import { ref, computed, onUnmounted } from 'vue'
import { TourGuide } from './TourGuide.js'

/**
 * Vue 3 composable for the tour guide.
 *
 * @param {import('./types').TourStep[]} steps
 * @returns {{
 *   currentStep: import('vue').ComputedRef<import('./types').TourStep|null>,
 *   currentIndex: import('vue').Ref<number>,
 *   isActive: import('vue').Ref<boolean>,
 *   isFirst: import('vue').ComputedRef<boolean>,
 *   isLast: import('vue').ComputedRef<boolean>,
 *   start: (index?: number) => void,
 *   next: () => void,
 *   prev: () => void,
 *   skip: () => void
 * }}
 */
export function useTourGuide(steps) {
  const isActive = ref(false)
  const currentIndex = ref(-1)
  const currentStep = ref(null)

  let tour = null

  const callbacks = {
    onStart() {
      isActive.value = true
    },
    onStepChange(step, index) {
      currentStep.value = step
      currentIndex.value = index
    },
    onComplete() {
      isActive.value = false
      currentStep.value = null
      currentIndex.value = -1
    }
  }

  const isFirst = computed(() => currentIndex.value === 0)
  const isLast = computed(() => currentIndex.value >= steps.length - 1)

  function start(startIndex = 0) {
    if (tour) tour.destroy()
    tour = new TourGuide(steps, callbacks)
    tour.start(startIndex)
  }

  function next() {
    if (tour && !isLast.value) {
      tour.next()
    } else if (tour) {
      tour.skip()
    }
  }

  function prev() {
    tour?.prev()
  }

  function skip() {
    tour?.skip()
  }

  onUnmounted(() => {
    tour?.destroy()
    tour = null
  })

  return {
    currentStep: computed(() => currentStep.value),
    currentIndex,
    isActive,
    isFirst,
    isLast,
    start,
    next,
    prev,
    skip
  }
}
