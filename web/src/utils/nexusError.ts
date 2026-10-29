/** 将 BFF / Core 错误转为用户可读的 Nexus 提示 */
export function formatNexusError(err: unknown, _hints?: { nexusConfigured?: boolean }): string {
  const raw = err instanceof Error ? err.message : String(err)
  const lower = raw.toLowerCase()

  if (
    lower.includes('nexus_core_url') ||
    lower.includes('nexus not configured') ||
    lower.includes('nexus client not configured')
  ) {
    return 'GrayDraft 未配置。'
  }

  if (
    lower.includes('connection refused') ||
    lower.includes('dial tcp') ||
    lower.includes('no such host') ||
    lower.includes('unreachable') ||
    lower.includes('502') ||
    lower.includes('bad gateway') ||
    lower.includes('fetch failed')
  ) {
    return '无法连接 GrayDraft Core。'
  }

  if (lower.includes('not found') && lower.includes('document')) {
    return '知识条目不存在。'
  }

  return raw
}
