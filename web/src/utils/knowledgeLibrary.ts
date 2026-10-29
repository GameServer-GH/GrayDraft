import type { KnowledgeEntryMeta } from '@/types/notebook'
import type { KnowledgeExtensions } from '@/types/knowledge'

export type KnowledgeListMode = 'grouped' | 'flat'

export type EntityFilterKey = 'subject' | 'version' | 'doc_type'

export interface KnowledgeSegmentGroup {
  key: string
  parentTitle: string
  segments: KnowledgeEntryMeta[]
  updatedAt: string
}

export interface KnowledgeListRow {
  kind: 'standalone' | 'group'
  id: string
  title: string
  updatedAt: string
  entry?: KnowledgeEntryMeta
  group?: KnowledgeSegmentGroup
}

const EXT_FILTER_KEYS: EntityFilterKey[] = ['subject', 'version', 'doc_type']

export function parseExtensions(raw: unknown): KnowledgeExtensions {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return {}
  return raw as KnowledgeExtensions
}

export function extString(ext: KnowledgeExtensions, key: EntityFilterKey): string {
  const v = ext[key]
  return typeof v === 'string' ? v.trim() : ''
}

export function segmentIndex(ext: KnowledgeExtensions): number | null {
  const v = ext.segment_index
  return typeof v === 'number' && Number.isFinite(v) ? v : null
}

export function segmentTotal(ext: KnowledgeExtensions): number | null {
  const v = ext.segment_total
  return typeof v === 'number' && Number.isFinite(v) ? v : null
}

export function parentTitle(ext: KnowledgeExtensions): string {
  const v = ext.parent_title
  return typeof v === 'string' ? v.trim() : ''
}

const SEGMENT_SUFFIX_RE = /[（(]\s*\d+\s*[/／]\s*\d+\s*[）)]\s*$/
const TRAILING_NOISE_RE = /[\s·\-—|/：:]+$/
const TITLE_STEM_SEPARATORS = [' · ', '：', ':', ' - ', ' — ', '｜', '|', ' / '] as const

function normalizeChildDocTitle(title: string): string {
  return title.trim().replace(SEGMENT_SUFFIX_RE, '').trim()
}

function trimParentTitleSuffix(s: string): string {
  return s.trim().replace(TRAILING_NOISE_RE, '').trim()
}

function titleStem(title: string): string {
  const normalized = normalizeChildDocTitle(title)
  if (!normalized) return ''
  let best = normalized
  for (const sep of TITLE_STEM_SEPARATORS) {
    const i = normalized.indexOf(sep)
    if (i > 0) {
      const part = normalized.slice(0, i).trim()
      if (part && (best === normalized || part.length < best.length)) best = part
    }
  }
  return trimParentTitleSuffix(best)
}

function commonTitleStem(titles: string[]): string {
  if (titles.length === 0) return ''
  const stems = titles.map(titleStem)
  const first = stems[0]
  if (!first) return ''
  return stems.every((s) => s === first) ? first : ''
}

function longestCommonPrefix(titles: string[]): string {
  if (titles.length === 0) return ''
  let prefix = titles[0]
  for (const t of titles.slice(1)) {
    while (t && !t.startsWith(prefix)) {
      if (!prefix) return ''
      prefix = prefix.slice(0, -1)
    }
  }
  return trimParentTitleSuffix(prefix)
}

function shortestPrefixOfAll(titles: string[]): string {
  const shortest = titles.reduce((a, b) => (a.length <= b.length ? a : b))
  if (titles.every((t) => t === shortest || t.startsWith(shortest))) {
    return trimParentTitleSuffix(shortest)
  }
  return ''
}

/** 由同组子知识标题归纳父标题（与 GrayDraft smartimport 规则一致） */
export function summarizeParentTitleFromChildTitles(titles: string[]): string {
  const cleaned: string[] = []
  const seen = new Set<string>()
  for (const raw of titles) {
    const t = normalizeChildDocTitle(raw)
    if (!t || seen.has(t)) continue
    seen.add(t)
    cleaned.push(t)
  }
  if (cleaned.length === 0) return ''
  if (cleaned.length === 1) return cleaned[0]
  const stem = commonTitleStem(cleaned)
  if (stem) return stem
  const prefix = longestCommonPrefix(cleaned)
  if (prefix.length >= 2) return prefix
  const shortest = shortestPrefixOfAll(cleaned)
  if (shortest) return shortest
  return cleaned[0]
}

export function deriveGroupParentTitle(segments: KnowledgeEntryMeta[]): string {
  const titles = segments.map((s) => s.title.trim()).filter(Boolean)
  const summarized = summarizeParentTitleFromChildTitles(titles)
  if (summarized) return summarized
  const stored = segments.map((s) => parentTitle(parseExtensions(s.extensions))).find(Boolean)
  return stored ?? titles[0] ?? ''
}

export function isSegmentEntry(entry: KnowledgeEntryMeta): boolean {
  const ext = parseExtensions(entry.extensions)
  return Boolean(parentTitle(ext)) && segmentIndex(ext) != null
}

export function matchesEntityFilter(entry: KnowledgeEntryMeta, filters: Partial<Record<EntityFilterKey, string>>): boolean {
  const ext = parseExtensions(entry.extensions)
  for (const key of EXT_FILTER_KEYS) {
    const want = filters[key]?.trim()
    if (!want) continue
    if (extString(ext, key) !== want) return false
  }
  return true
}

export function collectEntityOptions(entries: KnowledgeEntryMeta[]): Record<EntityFilterKey, string[]> {
  const out: Record<EntityFilterKey, Set<string>> = {
    subject: new Set(),
    version: new Set(),
    doc_type: new Set(),
  }
  for (const e of entries) {
    const ext = parseExtensions(e.extensions)
    for (const key of EXT_FILTER_KEYS) {
      const s = extString(ext, key)
      if (s) out[key].add(s)
    }
  }
  return {
    subject: [...out.subject].sort((a, b) => a.localeCompare(b)),
    version: [...out.version].sort((a, b) => a.localeCompare(b)),
    doc_type: [...out.doc_type].sort((a, b) => a.localeCompare(b)),
  }
}

export function buildSegmentGroups(entries: KnowledgeEntryMeta[]): KnowledgeSegmentGroup[] {
  const map = new Map<string, KnowledgeEntryMeta[]>()
  for (const e of entries) {
    const pt = parentTitle(parseExtensions(e.extensions))
    if (!pt || segmentIndex(parseExtensions(e.extensions)) == null) continue
    const list = map.get(pt) ?? []
    list.push(e)
    map.set(pt, list)
  }
  const groups: KnowledgeSegmentGroup[] = []
  for (const [groupKey, segs] of map) {
    segs.sort((a, b) => {
      const ia = segmentIndex(parseExtensions(a.extensions)) ?? 0
      const ib = segmentIndex(parseExtensions(b.extensions)) ?? 0
      return ia - ib
    })
    const latest = segs.reduce((max, s) => (s.updatedAt > max ? s.updatedAt : max), segs[0]?.updatedAt ?? '')
    const displayParent = deriveGroupParentTitle(segs) || groupKey
    groups.push({ key: groupKey, parentTitle: displayParent, segments: segs, updatedAt: latest })
  }
  groups.sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
  return groups
}

export function buildListRows(
  entries: KnowledgeEntryMeta[],
  mode: KnowledgeListMode,
): KnowledgeListRow[] {
  if (mode === 'flat') {
    return [...entries]
      .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
      .map((entry) => ({
        kind: 'standalone' as const,
        id: entry.id,
        title: entry.title,
        updatedAt: entry.updatedAt,
        entry,
      }))
  }

  const groupedIds = new Set<string>()
  const groups = buildSegmentGroups(entries)
  for (const g of groups) {
    for (const s of g.segments) groupedIds.add(s.id)
  }

  const rows: KnowledgeListRow[] = []
  for (const g of groups) {
    rows.push({
      kind: 'group',
      id: `group:${g.key}`,
      title: g.parentTitle,
      updatedAt: g.updatedAt,
      group: g,
    })
  }
  for (const e of entries) {
    if (groupedIds.has(e.id)) continue
    rows.push({
      kind: 'standalone',
      id: e.id,
      title: e.title,
      updatedAt: e.updatedAt,
      entry: e,
    })
  }
  rows.sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
  return rows
}

export interface BreadcrumbItem {
  label: string
  muted?: boolean
}

export function buildBreadcrumbs(
  title: string,
  ext: KnowledgeExtensions,
  segmentPartLabel: (index: string, total: string) => string,
  allEntries?: KnowledgeEntryMeta[],
): BreadcrumbItem[] {
  const items: BreadcrumbItem[] = []
  const topic = extString(ext, 'subject')
  const version = extString(ext, 'version')
  const docType = extString(ext, 'doc_type')
  let pt = parentTitle(ext)
  if (allEntries && segmentIndex(ext) != null && pt) {
    const storedKey = pt
    const siblings = allEntries.filter((e) => {
      const ex = parseExtensions(e.extensions)
      return parentTitle(ex) === storedKey && segmentIndex(ex) != null
    })
    if (siblings.length > 1) {
      pt = deriveGroupParentTitle(siblings)
    }
  }
  const idx = segmentIndex(ext)
  const total = segmentTotal(ext)

  if (topic) items.push({ label: topic })
  if (version) items.push({ label: version })
  if (docType) items.push({ label: docType })
  const groupParent = pt
  if (groupParent && groupParent !== title) items.push({ label: groupParent, muted: true })
  items.push({ label: title })
  if (idx != null && total != null && total > 1) {
    items.push({
      label: segmentPartLabel(String(idx), String(total)),
      muted: true,
    })
  }
  return items
}

export function buildRelatedQuery(title: string, content: string, ext: KnowledgeExtensions): string {
  const ctx = typeof ext.context_enhancement === 'string' ? ext.context_enhancement.trim() : ''
  if (ctx) return ctx
  const preview = content.trim().slice(0, 240)
  return [title.trim(), preview].filter(Boolean).join('\n')
}

export function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export interface QuickLinkSegment {
  text: string
  docId?: string
}

/** Split content into plain text and linkable title matches. */
export function splitContentWithQuickLinks(
  content: string,
  titlesById: { id: string; title: string }[],
  currentDocId: string,
): QuickLinkSegment[] {
  const titles = titlesById
    .filter((x) => x.id !== currentDocId && x.title.trim().length >= 2)
    .sort((a, b) => b.title.length - a.title.length)
  if (titles.length === 0) return [{ text: content }]

  const pattern = titles.map((x) => escapeRegExp(x.title.trim())).join('|')
  if (!pattern) return [{ text: content }]

  const re = new RegExp(`(${pattern})`, 'g')
  const titleMap = new Map(titles.map((x) => [x.title.trim(), x.id]))
  const parts = content.split(re)
  return parts.map((text) => {
    const docId = titleMap.get(text.trim())
    return docId ? { text, docId } : { text }
  })
}
