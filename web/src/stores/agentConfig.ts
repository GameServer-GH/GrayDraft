import { defineStore } from 'pinia'
import { requestWorkspaceSettingsSave } from '@/services/workspaceSettingsHub'
import {
  enrichWorkspaceContext,
  getMultimodalPromptDefaults,
  getWorkspaceContext,
  saveMultimodalWorkspacePrompt,
  saveWorkspaceContext,
} from '@/services/knowledge'
import type { WorkspaceContextProfile } from '@/services/workspace'

const DEFAULT_BASE_FALLBACK =
  '你是多模态内容提取助手。只提取图中可读的事实信息（名称、时间、数值、步骤、关系等），不要描述背景、配色、装饰、水印。直接输出 Markdown（禁止 JSON、禁止代码块围栏），第一行用 ### 标题，其后用列表写出关键字段（按实际内容选择）。'

const emptyProfile = (): WorkspaceContextProfile => ({
  description: '',
  entities: [],
  rules: [],
  negative_context: [],
})

/** 工作区 Agent 配置（非结构化提示词 + 结构化背景），持久化在 settings.json.agentConfig */
export const useAgentConfigStore = defineStore('agentConfig', {
  state: () => ({
    unstructuredPrompt: '',
    basePrompt: DEFAULT_BASE_FALLBACK,
    multimodalEnabled: false,
    loaded: false,
    contextProfile: emptyProfile(),
  }),
  actions: {
    applyFromServer(agentConfig: unknown, legacyFlat?: string) {
      let prompt = ''
      if (agentConfig && typeof agentConfig === 'object' && !Array.isArray(agentConfig)) {
        const o = agentConfig as Record<string, unknown>
        if (typeof o.unstructuredPrompt === 'string') {
          prompt = o.unstructuredPrompt
        }
        if (o.contextProfile && typeof o.contextProfile === 'object') {
          this.contextProfile = normalizeProfile(o.contextProfile as WorkspaceContextProfile)
        }
      }
      if (!prompt && typeof legacyFlat === 'string') {
        prompt = legacyFlat
      }
      this.unstructuredPrompt = prompt
    },

    isContextProfileConfigured(): boolean {
      const p = this.contextProfile
      return Boolean(p.description?.trim()) || (p.entities?.length ?? 0) > 0
    },

    needsWorkspaceContextSetup(): boolean {
      return !this.isContextProfileConfigured()
    },

    async refreshContextFromServer() {
      try {
        const ctx = await getWorkspaceContext()
        if (ctx) {
          this.contextProfile = normalizeProfile(ctx)
        }
      } catch {
        /* fallback: keep settings.json copy from applyFromServer */
      }
    },

    async refreshFromServer() {
      try {
        const res = await getMultimodalPromptDefaults()
        if (res.base_prompt) this.basePrompt = res.base_prompt
        if (res.workspace_prompt != null && res.workspace_prompt !== '') {
          this.unstructuredPrompt = res.workspace_prompt
        }
        this.multimodalEnabled = Boolean(res.enabled)
      } catch {
        /* ignore */
      }
      await this.refreshContextFromServer()
      this.loaded = true
    },

    setUnstructuredPrompt(v: string) {
      this.unstructuredPrompt = v
      requestWorkspaceSettingsSave()
    },

    setContextProfile(patch: Partial<WorkspaceContextProfile>) {
      this.contextProfile = normalizeProfile({ ...this.contextProfile, ...patch })
      requestWorkspaceSettingsSave()
    },

    setEntitiesFromText(text: string) {
      this.setContextProfile({ entities: splitLines(text) })
    },

    setRulesFromText(text: string) {
      this.setContextProfile({ rules: splitLines(text) })
    },

    setNegativeFromText(text: string) {
      this.setContextProfile({ negative_context: splitLines(text) })
    },

    async persistUnstructuredPromptNow() {
      await saveMultimodalWorkspacePrompt(this.unstructuredPrompt)
    },

    async persistContextProfileNow() {
      await saveWorkspaceContext(this.contextProfile)
    },

    /** 用户一句话 → LLM 扩展结构化画像 → 写入 settings 与 Core */
    async enrichAndSaveFromBrief(brief: string) {
      const profile = await enrichWorkspaceContext(brief)
      this.contextProfile = normalizeProfile(profile)
      requestWorkspaceSettingsSave()
    },

    toSettingsFields(): {
      agentConfig: {
        unstructuredPrompt: string
        contextProfile: WorkspaceContextProfile
      }
      multimodalWorkspacePrompt: string
    } {
      const text = this.unstructuredPrompt
      return {
        agentConfig: {
          unstructuredPrompt: text,
          contextProfile: normalizeProfile(this.contextProfile),
        },
        multimodalWorkspacePrompt: text,
      }
    },
  },
})

function normalizeProfile(p: WorkspaceContextProfile): WorkspaceContextProfile {
  return {
    description: (p.description ?? '').trim(),
    entities: (p.entities ?? []).map((s) => s.trim()).filter(Boolean),
    rules: (p.rules ?? []).map((s) => s.trim()).filter(Boolean),
    negative_context: (p.negative_context ?? []).map((s) => s.trim()).filter(Boolean),
  }
}

function splitLines(text: string): string[] {
  return text
    .split(/[\n,，、]+/)
    .map((s) => s.trim())
    .filter(Boolean)
}

/** @deprecated use useAgentConfigStore */
export const useMultimodalPromptStore = useAgentConfigStore
