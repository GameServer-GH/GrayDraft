/** 将 `Hello {name}` 中的占位符替换为 params.name */
export function interpolate(template: string, params?: Record<string, string>): string {
  if (!params) return template
  return template.replace(/\{(\w+)\}/g, (_, key: string) =>
    Object.prototype.hasOwnProperty.call(params, key) ? params[key]! : `{${key}}`,
  )
}
