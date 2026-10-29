/**
 * @typedef {Object} TourStep
 * @property {string|Function|Element} targetSelector
 * @property {number} [spotlightPadding=8]
 * @property {string} title
 * @property {string} content
 * @property {'top'|'bottom'|'left'|'right'|'auto'} [cardPlacement='auto']
 * @property {number} [cardOffset=16]
 */

/**
 * @typedef {Object} TourCallbacks
 * @property {() => void} [onStart]
 * @property {(step: TourStep, index: number) => void} [onStepChange]
 * @property {() => void} [onComplete]
 */

export {}
