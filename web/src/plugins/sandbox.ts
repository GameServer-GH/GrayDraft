/**
 * iframe 生命周期 — 方案 plugins/sandbox.ts
 * sandbox: allow-same-origin allow-scripts（方案要求，实现时严格遵守）
 */
export function createPluginSandbox(_iframe: HTMLIFrameElement): void {
  void _iframe
}
