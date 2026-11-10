import { defineStore } from 'pinia'
import { requestWorkspaceSettingsSave } from '@/services/workspaceSettingsHub'

/** 预设供应商类型（用户添加一条配置时选择） */
export const VENDOR_KIND_IDS = ['anthropic', 'openai', 'google', 'deepseek', 'zhipu', 'intern'] as const
export type VendorKindId = (typeof VENDOR_KIND_IDS)[number]

export function isVendorKindId(v: string): v is VendorKindId {
  return (VENDOR_KIND_IDS as readonly string[]).includes(v)
}

export type ConfiguredModel = {
  id: string
  /** 对外模型名，如 deepseek-v4-flash */
  name: string
  enabled: boolean
}

export type ConfiguredVendor = {
  id: string
  kindId: VendorKindId
  apiKey: string
  enabled: boolean
  models: ConfiguredModel[]
}

function newId(): string {
  try {
    return crypto.randomUUID()
  } catch {
    return `id_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`
  }
}

function safeParseVendors(data: unknown): ConfiguredVendor[] {
  if (!Array.isArray(data)) return []
  try {
    const out: ConfiguredVendor[] = []
    for (const row of data) {
      if (!row || typeof row !== 'object') continue
      const o = row as Record<string, unknown>
      const id = typeof o.id === 'string' ? o.id : newId()
      const kindId = typeof o.kindId === 'string' && isVendorKindId(o.kindId) ? o.kindId : null
      if (!kindId) continue
      const apiKey = typeof o.apiKey === 'string' ? o.apiKey : ''
      const enabled = typeof o.enabled === 'boolean' ? o.enabled : true
      const modelsRaw = Array.isArray(o.models) ? o.models : []
      const models: ConfiguredModel[] = []
      for (const m of modelsRaw) {
        if (!m || typeof m !== 'object') continue
        const mo = m as Record<string, unknown>
        const mid = typeof mo.id === 'string' ? mo.id : newId()
        const name = typeof mo.name === 'string' ? mo.name.trim() : ''
        if (!name) continue
        const me = typeof mo.enabled === 'boolean' ? mo.enabled : true
        models.push({ id: mid, name, enabled: me })
      }
      out.push({ id, kindId, apiKey, enabled, models })
    }
    return out
  } catch {
    return []
  }
}

/** Agent 下拉选项值：自动 或 「配置实例 id : 模型实例 id」 */
export function agentModelOptionValue(vendorId: string, modelId: string): string {
  return `${vendorId}:${modelId}`
}

export function parseAgentModelOptionValue(v: string): { vendorId: string; modelId: string } | null {
  const i = v.indexOf(':')
  if (i <= 0) return null
  return { vendorId: v.slice(0, i), modelId: v.slice(i + 1) }
}

export const useAgentModelConfigStore = defineStore('agentModelConfig', {
  state: () => ({
    vendors: [] as ConfiguredVendor[],
  }),
  getters: {
    /** 已启用供应商下已启用的模型，供 Agent 下拉使用 */
    enabledPickerEntries(): { value: string; vendorId: string; kindId: VendorKindId; modelName: string }[] {
      const list: { value: string; vendorId: string; kindId: VendorKindId; modelName: string }[] = []
      for (const v of this.vendors) {
        if (!v.enabled) continue
        for (const m of v.models) {
          if (!m.enabled) continue
          list.push({
            value: agentModelOptionValue(v.id, m.id),
            vendorId: v.id,
            kindId: v.kindId,
            modelName: m.name,
          })
        }
      }
      return list
    },
  },
  actions: {
    /** 已由工作区 OpenWorkspace 恢复；保留空实现避免旧调用处报错。 */
    hydrate() {},
    applyFromServer(data: unknown) {
      this.vendors = safeParseVendors(data)
    },
    persist() {
      requestWorkspaceSettingsSave()
    },
    addVendor(kindId: VendorKindId, apiKey: string) {
      const key = apiKey.trim()
      // 书生未填 Key 时留空，由服务端使用内置加密密钥。
      if (!key && kindId !== 'intern') return
      this.vendors.push({
        id: newId(),
        kindId,
        apiKey: key,
        enabled: true,
        models: [],
      })
      this.persist()
    },
    removeVendor(vendorId: string) {
      this.vendors = this.vendors.filter((v) => v.id !== vendorId)
      this.persist()
    },
    setVendorEnabled(vendorId: string, enabled: boolean) {
      const v = this.vendors.find((x) => x.id === vendorId)
      if (!v) return
      v.enabled = enabled
      this.persist()
    },
    setVendorApiKey(vendorId: string, apiKey: string) {
      const v = this.vendors.find((x) => x.id === vendorId)
      if (!v) return
      v.apiKey = apiKey
      this.persist()
    },
    addModel(vendorId: string, name: string) {
      const trimmed = name.trim()
      if (!trimmed) return
      const v = this.vendors.find((x) => x.id === vendorId)
      if (!v) return
      v.models.push({
        id: newId(),
        name: trimmed,
        enabled: true,
      })
      this.persist()
    },
    removeModel(vendorId: string, modelId: string) {
      const v = this.vendors.find((x) => x.id === vendorId)
      if (!v) return
      v.models = v.models.filter((m) => m.id !== modelId)
      this.persist()
    },
    setModelEnabled(vendorId: string, modelId: string, enabled: boolean) {
      const v = this.vendors.find((x) => x.id === vendorId)
      if (!v) return
      const m = v.models.find((x) => x.id === modelId)
      if (!m) return
      m.enabled = enabled
      this.persist()
    },
    selectionIsValid(value: string): boolean {
      if (value === 'auto') return true
      const parsed = parseAgentModelOptionValue(value)
      if (!parsed) return false
      const v = this.vendors.find((x) => x.id === parsed.vendorId)
      if (!v || !v.enabled) return false
      const m = v.models.find((x) => x.id === parsed.modelId)
      return Boolean(m && m.enabled)
    },
  },
})
