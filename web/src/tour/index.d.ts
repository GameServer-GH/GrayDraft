declare module '@/tour' {
  import type { TourStep } from '@/tour/types'
  export { TourGuide } from '@/tour/TourGuide'
  export { useTourGuide } from '@/tour/useTourGuide'
  export { default as TourOverlay } from '@/tour/TourOverlay.vue'
  export function getGrayDraftTourSteps(callbacks?: { openSidebar?: () => void; openAgent?: () => void }): TourStep[]
  export function shouldShowGrayDraftTour(): boolean
  export function markGrayDraftTourSeen(): void
  export function resetGrayDraftTour(): void
}

declare module '@/tour/TourOverlay.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{
    step?: import('@/tour/types').TourStep | null
    stepIndex?: number
    totalSteps?: number
    isFirst?: boolean
    isLast?: boolean
    targetSelector?: string | Function | Element | null
    spotlightPadding?: number
    cardPlacement?: string
    cardOffset?: number
  }, {}, any>
  export default component
}
