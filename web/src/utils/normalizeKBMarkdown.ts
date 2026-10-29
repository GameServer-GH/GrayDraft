/** Fix common LLM Markdown spacing (glued headings, numbered lists, bullets, tables). Preserves MM.DD date ranges. */

const MD_DATE_RANGE = /\d{2}\.\d{2}(?:\s*[-–—~～]\s*\d{2}\.\d{2})?/g
const MD_HEADING_SPACE = /(^#{1,6})([^\s#\n])/gm
const MD_HEADING_GLUE = /([^\n#\s])(#{1,6}\s*)/g
/** List "1.xxx" — (?!\d) skips dates like 06.19 */
const MD_NUM_DOT_SPACE = /(\d{1,2})\.(?!\d)([^\s\n])/g
const MD_NUM_GLUE = /([^\n\d])(\d{1,2})\.(?!\d)(\s*)/g
const MD_NUM_AFTER_BOLD = /(\*\*)(\d{1,2})\.(?!\d)(\s*)/g
const MD_BULLET_GLUE = /([^\n\s\-*\d.])(-\s+)/g
const MD_BULLET_AFTER_BOLD = /(\*\*)(-)([^\s\n])/g
const MD_EXCESS_NL = /\n{3,}/g
const MD_TABLE_GLUE = /\|\|/g
/** ## heading_text|col1|col2 — heading fused with table header row */
const MD_HEADING_TABLE_GLUE = /^(#{1,6}\s+)([^|\n]+?)(\|[^\n]+)$/gm

/** Strip trailing empty columns from Markdown tables when every data row
 *  has a trailing empty cell, keeping column count consistent. */
function normalizeTableColumns(source: string): string {
  const lines = source.split('\n')
  const result: string[] = []
  let i = 0
  while (i < lines.length) {
    const line = lines[i]
    if (!/^\|.*\|\s*$/.test(line)) {
      result.push(line)
      i++
      continue
    }
    // Collect consecutive table lines
    const block: string[] = []
    while (i < lines.length && /^\|.*\|\s*$/.test(lines[i])) {
      block.push(lines[i])
      i++
    }
    if (block.length < 2) {
      result.push(...block)
      continue
    }
    // Find separator row index (|---|...)
    const sepIdx = block.findIndex((l) => /^\|\s*[-:]+\s*/.test(l))
    if (sepIdx < 1) { result.push(...block); continue }
    // Count columns in separator (reliable reference)
    const sepCols = block[sepIdx].split('|').filter((c) => c.trim() !== '').length + 1
    // Check if every data row's last cell is empty (whitespace only)
    const dataRows = block.filter((_, idx) => idx !== sepIdx)
    const allHaveTrailingEmpty = dataRows.every((row) => {
      const cells = row.split('|')
      // Last meaningful cell should be empty
      return cells.length >= 2 && cells[cells.length - 2].trim() === ''
    })
    if (allHaveTrailingEmpty && sepCols >= 3) {
      // Remove trailing empty column: drop the second-to-last element from split
      result.push(...block.map((row) => {
        const cells = row.split('|')
        // Remove the empty cell before the trailing |
        cells.splice(cells.length - 2, 1)
        return cells.join('|')
      }))
    } else {
      result.push(...block)
    }
  }
  return result.join('\n')
}

function shieldDates(source: string): { text: string; dates: string[] } {
  const dates: string[] = []
  const text = source.replace(MD_DATE_RANGE, (d) => {
    dates.push(d)
    return `\u0000DATE${dates.length - 1}\u0000`
  })
  return { text, dates }
}

function unshieldDates(text: string, dates: string[]): string {
  let s = text
  dates.forEach((d, i) => {
    s = s.split(`\u0000DATE${i}\u0000`).join(d)
  })
  return s
}

export function normalizeKBMarkdown(source: string): string {
  let s = source.trim()
  if (!s) return s
  const shielded = shieldDates(s)
  s = shielded.text
  s = s.replace(MD_HEADING_SPACE, '$1 $2')
  s = s.replace(MD_HEADING_GLUE, '$1\n\n$2')
  s = s.replace(MD_NUM_DOT_SPACE, '$1. $2')
  s = s.replace(MD_NUM_GLUE, '$1\n\n$2.$3')
  s = s.replace(MD_NUM_AFTER_BOLD, '$1\n\n$2.$3')
  s = s.replace(MD_BULLET_AFTER_BOLD, '$1\n$2 $3')
  s = s.replace(MD_BULLET_GLUE, '$1\n$2')
  s = s.replace(MD_EXCESS_NL, '\n\n')

  // Split glued heading+table: "## title|col1|col2" → "## title\n\n|col1|col2"
  s = s.replace(MD_HEADING_TABLE_GLUE, '$1$2\n\n$3')

  // Table row repair: split "||" collapsed rows into separate lines
  s = s.replace(MD_TABLE_GLUE, '\n|')

  // Normalize table column counts: strip truly-empty trailing columns from all rows
  s = normalizeTableColumns(s)

  return unshieldDates(s, shielded.dates).trim()
}
