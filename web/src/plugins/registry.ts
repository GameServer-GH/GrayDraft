/**
 * 扩展点注册 — 方案 plugins/registry.ts
 */
const registry = new Map<string, unknown>()

export function registerExtensionPoint(name: string, impl: unknown): void {
  registry.set(name, impl)
}

export function getExtensionPoint<T>(name: string): T | undefined {
  return registry.get(name) as T | undefined
}
