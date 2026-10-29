import type { KnowledgeExtensions } from '@/types/knowledge'

export interface ExtensionFieldRow {
  key: string
  value: string
}

export function rowsFromExtensions(raw: unknown): ExtensionFieldRow[] {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return []
  const rows: ExtensionFieldRow[] = []
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    const k = key.trim()
    if (!k) continue
    if (value == null) {
      rows.push({ key: k, value: '' })
      continue
    }
    if (typeof value === 'string') {
      rows.push({ key: k, value })
      continue
    }
    if (typeof value === 'number' || typeof value === 'boolean') {
      rows.push({ key: k, value: String(value) })
      continue
    }
    try {
      rows.push({ key: k, value: JSON.stringify(value) })
    } catch {
      rows.push({ key: k, value: String(value) })
    }
  }
  return rows
}

/** Build extensions object for API; skips empty keys. Values stay strings (Worker stores jsonb). */
export function extensionsFromRows(rows: ExtensionFieldRow[]): KnowledgeExtensions | undefined {
  const out: KnowledgeExtensions = {}
  for (const row of rows) {
    const key = row.key.trim()
    if (!key) continue
    out[key] = row.value.trim()
  }
  return Object.keys(out).length > 0 ? out : undefined
}
