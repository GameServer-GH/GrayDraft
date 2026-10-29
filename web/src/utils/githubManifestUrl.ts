/**
 * 将 GitHub 上的 manifest 地址解析为可 fetch 的 raw URL。
 * 支持 raw.githubusercontent.com；github.com 仅支持 /blob/{单段分支或标签}/路径（多段 ref 请直接粘贴 Raw 链接）。
 */
export function resolveGithubManifestUrl(input: string): string | null {
  const trimmed = input.trim()
  if (!trimmed) return null
  let u: URL
  try {
    u = new URL(trimmed)
  } catch {
    return null
  }
  if (u.protocol !== 'http:' && u.protocol !== 'https:') return null

  const host = u.hostname.replace(/^www\./, '').toLowerCase()

  if (host === 'raw.githubusercontent.com') {
    return u.href
  }

  if (host !== 'github.com') return null

  const segs = u.pathname.split('/').filter(Boolean)
  if (segs.length < 5) return null

  const owner = segs[0]
  const repo = segs[1]
  const mode = segs[2]
  const ref = segs[3]
  const filePath = segs.slice(4).join('/')

  if (!owner || !repo || !ref || !filePath) return null
  if (mode !== 'blob' && mode !== 'raw') return null
  if (ref === 'refs' || ref.includes('/')) return null

  return `https://raw.githubusercontent.com/${owner}/${repo}/${ref}/${filePath}`
}
