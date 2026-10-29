/**
 * Framework-agnostic Tour Guide engine.
 * Uses 4-panel overlay (top/right/bottom/left) so clicks pass through the spotlight hole.
 */

const OVERLAY_Z = 9000

class TourGuide {
  constructor(steps, callbacks = {}) {
    this.steps = steps
    this.callbacks = callbacks
    this.currentIndex = -1
    this.active = false
    this._resizeHandler = null
    this._mutationObserver = null
    this._panels = {}
    this._guardEl = null
    this._retryTimer = null
  }

  start(startIndex = 0) {
    if (this.steps.length === 0) return
    this._createPanels()
    this._bindEvents()
    this._goTo(Math.min(startIndex, this.steps.length - 1))
    this.active = true
    this.callbacks.onStart?.()
  }

  next() {
    if (this.currentIndex < this.steps.length - 1) {
      this._clearRetry()
      this._goTo(this.currentIndex + 1)
    } else {
      this.skip()
    }
  }

  prev() {
    if (this.currentIndex > 0) {
      this._clearRetry()
      this._goTo(this.currentIndex - 1)
    }
  }

  skip() {
    this._finish()
  }

  destroy() {
    this._unbindEvents()
    this._clearRetry()
    Object.values(this._panels).forEach(p => {
      if (p && p.parentNode) p.parentNode.removeChild(p)
    })
    if (this._guardEl && this._guardEl.parentNode) this._guardEl.parentNode.removeChild(this._guardEl)
    this._guardEl = null
    this._panels = {}
    this.active = false
  }

  getCurrentStep() { return this.steps[this.currentIndex] || null }
  isFirst() { return this.currentIndex === 0 }
  isLast() { return this.currentIndex >= this.steps.length - 1 }

  // ---- internals ----

  _goTo(index) {
    this.currentIndex = index
    const step = this.steps[index]
    if (!step) return

    // run step enter action (e.g. auto-open dialog)
    if (step.stepEnterAction) step.stepEnterAction()

    this.callbacks.onStepChange?.(step, index)

    const targetEl = this._resolveTarget(step)
    if (targetEl) {
      this._showSpotlight(targetEl, step.spotlightPadding ?? 8)
    } else {
      // keep previous panels visible while retrying async target
      this._retryFindTarget(step, index)
    }
  }

  _retryFindTarget(step, index, attempt = 0) {
    this._clearRetry()
    if (attempt >= 30 || !this.active || this.currentIndex !== index) return
    this._retryTimer = setTimeout(() => {
      if (!this.active || this.currentIndex !== index) return
      const el = this._resolveTarget(step)
      if (el) {
        this._showSpotlight(el, step.spotlightPadding ?? 8)
      } else {
        this._retryFindTarget(step, index, attempt + 1)
      }
    }, 300)
  }

  _clearRetry() {
    if (this._retryTimer) { clearTimeout(this._retryTimer); this._retryTimer = null }
  }

  _resolveTarget(step) {
    if (!step.targetSelector) return null
    if (typeof step.targetSelector === 'string') return document.querySelector(step.targetSelector)
    if (typeof step.targetSelector === 'function') return step.targetSelector()
    if (step.targetSelector instanceof Element) return step.targetSelector
    return null
  }

  _createPanels() {
    if (Object.keys(this._panels).length) return
    const base = `position:fixed;z-index:${OVERLAY_Z};background:rgba(0,0,0,0.55);pointer-events:all;transition:all 0.35s cubic-bezier(0.22,1,0.36,1);`
    ;['top','right','bottom','left'].forEach(pos => {
      const panel = document.createElement('div')
      panel.className = `tour-overlay-panel tour-overlay-${pos}`
      panel.style.cssText = base + 'top:0;left:0;width:0;height:0;'
      document.body.appendChild(panel)
      this._panels[pos] = panel
    })
    // spotlight guard: transparent white overlay in the hole to block interaction
    const guard = document.createElement('div')
    guard.className = 'tour-spotlight-guard'
    guard.style.cssText = `position:fixed;z-index:${OVERLAY_Z + 1};pointer-events:all;background:rgba(255,255,255,0.3);transition:all 0.35s cubic-bezier(0.22,1,0.36,1);top:0;left:0;width:0;height:0;border-radius:12px;`
    document.body.appendChild(guard)
    this._guardEl = guard
  }

  _showSpotlight(el, padding = 8) {
    const rect = el.getBoundingClientRect()
    const x = rect.left - padding
    const y = rect.top - padding
    const w = rect.width + padding * 2
    const h = rect.height + padding * 2
    const vw = window.innerWidth
    const vh = window.innerHeight

    const styleTop  = `;top:0;left:0;width:100vw;height:${Math.max(0,y)}px;`
    const styleBot  = `;top:${y+h}px;left:0;width:100vw;height:${Math.max(0,vh-y-h)}px;`
    const styleLeft = `;top:${y}px;left:0;width:${Math.max(0,x)}px;height:${h}px;`
    const styleRight= `;top:${y}px;left:${x+w}px;width:${Math.max(0,vw-x-w)}px;height:${h}px;`

    const pfx = 'position:fixed;z-index:' + OVERLAY_Z + ';background:rgba(0,0,0,0.55);pointer-events:all;transition:all 0.35s cubic-bezier(0.22,1,0.36,1);'

    if (this._panels.top) this._panels.top.style.cssText = pfx + styleTop
    if (this._panels.bottom) this._panels.bottom.style.cssText = pfx + styleBot
    if (this._panels.left) this._panels.left.style.cssText = pfx + styleLeft
    if (this._panels.right) this._panels.right.style.cssText = pfx + styleRight
    if (this._guardEl) this._guardEl.style.cssText = `position:fixed;z-index:${OVERLAY_Z + 1};pointer-events:all;background:rgba(255,255,255,0.30);transition:all 0.35s cubic-bezier(0.22,1,0.36,1);top:${y}px;left:${x}px;width:${w}px;height:${h}px;border-radius:12px;`
  }

  _hidePanels() {
    Object.values(this._panels).forEach(p => {
      if (p) p.style.cssText = p.style.cssText.split(';position:')[0] + ';position:fixed;z-index:' + OVERLAY_Z + ';background:rgba(0,0,0,0.55);pointer-events:all;transition:all 0.35s cubic-bezier(0.22,1,0.36,1);top:0;left:0;width:0;height:0;'
    })
    if (this._guardEl) this._guardEl.style.cssText = this._guardEl.style.cssText.split(';top:')[0] + ';top:0;left:0;width:0;height:0;'
  }

  _updateSpotlight() {
    const step = this.steps[this.currentIndex]
    if (!step) return
    const targetEl = this._resolveTarget(step)
    if (targetEl) {
      this._showSpotlight(targetEl, step.spotlightPadding ?? 8)
    }
  }

  _onResize = () => { if (this.active) this._updateSpotlight() }

  _onMutation = (mutations) => { if (!this.active) return; const isOwn = mutations.some(m => m.target.classList?.contains('tour-overlay-panel')); if (isOwn) return; this._updateSpotlight() }

  _bindEvents() {
    this._resizeHandler = this._onResize.bind(this)
    window.addEventListener('resize', this._resizeHandler)
    this._mutationObserver = new MutationObserver(this._onMutation.bind(this))
    this._mutationObserver.observe(document.body, {
      childList: true, subtree: true,
      attributes: true, attributeFilter: ['style', 'class']
    })
  }

  _unbindEvents() {
    if (this._resizeHandler) { window.removeEventListener('resize', this._resizeHandler); this._resizeHandler = null }
    if (this._mutationObserver) { this._mutationObserver.disconnect(); this._mutationObserver = null }
  }

  _finish() {
    this._clearRetry()
    this.callbacks.onComplete?.()
    this.destroy()
  }
}

export { TourGuide }
export default TourGuide
